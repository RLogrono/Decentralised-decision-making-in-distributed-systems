package main

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var m map[string]*nodo
var tprcc map[string]*timeProcess
var servers, inters, pys []string
var pcCola chan string
var registro map[int]*cabecera = make(map[int]*cabecera)

func Check(err error) {
	if err != nil {
		panic(err)
	}
}

/* Variables globales */
const (
	Chunksize int     = 20000000 //Tamaño fijo
	eps       float64 = 2        //Tolerancia
	filePort  string  = ":9544"  //Puerto para la transferencia de archivos
)

var infoServer string = "10.0.5.2:9545" //Direccion IP del servidor de informacion
//var infoServer string = "127.0.0.1:9545"

type chunk struct {
	nombre string
	part   int
	total  int
	f      *os.File
}

type cabecera struct {
	flag        string    //Info, file, get_info
	sizeORqueue int64     //Tamaño del archivo o tamaño de cola
	origen      string    //Qué video es
	Tdl         time.Time //Deadline
	tpiece      float32   //Duracion de la parte
	trace       string
}

type recta struct {
	m float32 //pendiente
	n float32 //ordenada en el origen
}

func (r recta) y(x float32) (out float32) {
	out = r.m*x + r.n
	return
}

func (r recta) String() string {
	return "y = " + fmt.Sprint(r.m) + "x + " + fmt.Sprint(r.n) + ";"
}

type video struct {
	name string
	size int
	dur  float32
	fps  float32
}

/*
	Manda la infomarcion propia del nodo al nodo de medidas.
	Como manda datos concretos no hace falta mandar una
	estructura tipo cabecera.
*/

type info_message struct {
	valor int
	tag   string
	hora  time.Time
}

func (im info_message) String() string {
	text := ""
	text += fmt.Sprint(im.valor) + " " + im.tag + " " + im.hora.String()
	return text
}

/*
Establece una conexión TCP con el servidor de medidas
y le pide la ocupacion de un nodo en concreto.

Un nodo nunca debe hacer get_info(m["me"]), ya que estaria sobreescribiendo
su propio valor de ocupacion. De toda formas da error porque un nodo se tiene
a si mismo como "me", no como dirIP.
*/

func send_while_reciving(ch chan *chunk, id int, tag string) {
	cab := registro[id]

	b1 := make([]byte, Chunksize)
	//servAddr := "localhost:9545" //Dirección IP del siguiente nodo
	tcpAddr, err := net.ResolveTCPAddr("tcp", m[tag].dirIP)
	Check(err)
	con, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		fmt.Println("Error al reenviar, añado el video a la cola local:\n", err)
		name := build_file(ch, id)
		Prcss_video(name, id)
		return
	}
	defer con.Close()
	cab.trace += tag + ";"
	send_state(con, *cab)
	for i := range ch { //Comienza el reenvio
		texto := fmt.Sprint(cab.origen, " ", i.part, " ", i.total, "\n")
		_, e := con.Write([]byte(texto))
		if e != nil { // si entra es que hay un: Error de desconexion
			fmt.Println(e)
			con.Close()
			con, err = net.DialTCP("tcp", nil, tcpAddr)
			if err != nil {
				panic(err)
			}
			con.Write([]byte(texto))
		}
		i.f.Seek(0, 0)
		for { //Manda el archivo
			n, e := i.f.Read(b1)
			if e != nil {
				fmt.Println(e)
				break
			}
			con.Write(b1[:n])
		}
		i.Close()
	}
}

func get_info(target string) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", infoServer)
	Check(err)
	con, err := net.DialTCP("tcp", nil, tcpAddr)
	Check(err)
	send_state(con, cabecera{"get_info", 0, target, time.Now().Local(), 0, ""})
	dec := gob.NewDecoder(con)
	var im info_message
	err = dec.Decode(&im)
	if err != nil {
		fmt.Println("[ERROR code 007]:", err)
	}
	fmt.Println("Obtengo de", target+":", im.valor, im.hora)
	m[target].ocupados = im.valor
	m[target].tslot = im.hora
	fmt.Println("[2]Obtengo de", target+":", m[target].ocupados, m[target].tslot)
}

func send_info() {
	udpAddr, err := net.ResolveUDPAddr("udp", infoServer)
	Check(err)
	con, err := net.DialUDP("udp", nil, udpAddr)
	Check(err)
	var im info_message = info_message{m["me"].ocupados, m["me"].tag, time.Now().Local()}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(im); err != nil {
		fmt.Println("Error en la codificacion :(")
		panic(err)
	}
	_, err = con.Write(buf.Bytes())
	if err != nil {
		panic(err)
	}
}

func send_result(cab *cabecera) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", infoServer)
	Check(err)
	con, err := net.DialTCP("tcp", nil, tcpAddr)
	Check(err)
	send_state(con, cabecera{"results", 0, cab.origen, time.Now().Local(), 0, cab.trace})
	_, err = con.Write([]byte(cab.Tdl.Sub(time.Now().Local()).String() + "\n"))
	if err != nil {
		fmt.Println("[ERROR]: Error al mandar los resultados.")
	}
	con.Close()
}

func build_file(ch chan *chunk, id int) (name string) {

	name = fmt.Sprint(id)
	f, e := os.Create(name)
	Check(e)
	defer f.Close()

	for i := range ch { //Escribo en el archivo definitivo los chunks
		i.f.Seek(0, 0)
		leerescribir(i.f, f)
		i.Close()
	}
	return
}

func send_state(con net.Conn, cab cabecera) {
	enc := gob.NewEncoder(con)
	enc.Encode(cab)
}
func recive_state(rdr *bufio.Reader) (cab cabecera) {
	dec := gob.NewDecoder(rdr)
	err := dec.Decode(&cab)
	if err != nil {
		fmt.Println(err)
	}
	return
}

func (im info_message) MarshalBinary() ([]byte, error) {
	// A simple encoding: plain text.
	var b bytes.Buffer
	fmt.Fprintln(&b, im.tag, im.valor)
	bb, err := im.hora.GobEncode()
	if err != nil {
		panic(err)
	}
	return append(b.Bytes(), bb...), nil
}

func (im *info_message) UnmarshalBinary(data []byte) error {
	// A simple encoding: plain text.
	b := bytes.NewBuffer(data)
	_, err := fmt.Fscanln(b, &im.tag, &im.valor)
	if err == nil {
		im.hora.GobDecode(b.Bytes())
	} else {
		fmt.Println("[ERROR]: Unmarshal info_message", string(data))
	}
	return err
}

func (c cabecera) MarshalBinary() ([]byte, error) {
	// A simple encoding: plain text.
	var b bytes.Buffer
	fmt.Fprintln(&b, c.flag, c.sizeORqueue, c.origen, c.tpiece, c.trace)
	bb, err := c.Tdl.GobEncode()
	if err != nil {
		panic(err)
	}
	return append(b.Bytes(), bb...), nil
}

// UnmarshalBinary modifies the receiver so it must take a pointer receiver.
func (c *cabecera) UnmarshalBinary(data []byte) error {
	b := bytes.NewBuffer(data)
	_, err := fmt.Fscanln(b, &c.flag, &c.sizeORqueue, &c.origen, &c.tpiece, &c.trace)
	if err == nil {
		c.Tdl.GobDecode(b.Bytes())
	} else {
		fmt.Println("[ERROR]: Unmarshal cabecera,", err)
	}
	return err
}

//Todos los tiempos en milisegundos
/*
El tiempo de procesado en el servidor no se usa, ya que
segun esta ahora se procesa en el servidor como ultimo
recurso si no da tiempo a hacerlo en local o nodo intermedio
*/
type timeProcess struct {
	server recta
	inter  recta
	pi     recta
}

func (tp timeProcess) String() string {
	return fmt.Sprint("\nServer", tp.server, "\nInter", tp.inter, "\npi", tp.pi, "\n")
}

/* Estructura de nodo */

type nodo struct {
	dirIP     string
	tag       string
	tipo      string
	slots     int
	tslot     time.Time
	bandwidth int
	tbandwidt time.Time
	ocupados  int
	tmedio    int
}

func SetNodo(s string) (n *nodo) {

	temp := strings.Split(s[:len(s)-1], " ")
	i, err := strconv.Atoi(temp[2])
	Check(err)
	n = &nodo{temp[0], temp[3], temp[1], i, time.Now().Local(), 2000000, time.Now().Local(), 0, 0}
	return
}

/*
Lee los archivos de configuracion para crear las estructuras de datos que fijan el comportamiento particular
del nodo.

1- node.conf: añade a un mapa datos de tipo nodo todos los vecinos y a él mismo.
2- Crea tres vectores de strings que contienen las direcciones de los nodos ordenados por tipo: server, intermedio, pi.
3- time.conf: configura un mapa de tiempos de procesado para realizar las aproximaciones en la decision.
*/

func auto_config() (nodos map[string]*nodo, tiempos map[string]*timeProcess) {
	f, err := os.Open("node.conf")
	if err != nil {
		fmt.Println("[ERROR]Falta el archivo: 'node.conf'")
		os.Exit(1)
	}
	nodos = make(map[string]*nodo)
	b1 := make([]byte, 20000)
	n, _ := f.Read(b1)
	f.Close()
	vector := strings.Split(string(b1[:n]), "\n")
	nodos["me"] = SetNodo(vector[0])
	nodos[nodos["me"].tag] = nodos["me"]
	for i := 1; i < len(vector); i++ {
		if len(vector[i]) <= 2 {
			break
		}
		temp := SetNodo(vector[i])
		if temp.tipo == "mnode" {
			infoServer = temp.dirIP
		} else {
			nodos[temp.tag] = temp
			if temp.tipo == "pi" {
				pys = append(pys, temp.tag)
			} else if temp.tipo == "inter" {
				inters = append(inters, temp.tag)
			} else if temp.tipo == "server" {
				servers = append(servers, temp.tag)
			} else {
				panic("[ERROR] Archivo de configuracion errorneo: tipo desconocido")
			}
		}
	}
	f.Close()
	s, err := ioutil.ReadFile("time.conf")
	if err != nil {
		fmt.Println("[ERROR]Falta el archivo: 'time.conf'")
		os.Exit(1)
	}
	vector2 := strings.Split(string(s), "\n")
	tiempos = make(map[string]*timeProcess)
	for i := 0; i < len(vector2); i++ {
		vector2[i] = strings.ReplaceAll(vector2[i], "\r", "")
		temp := strings.Split(vector2[i], " ")
		var r [3]recta
		for j := 1; j < 4; j++ {
			num1, err := strconv.ParseFloat(temp[j*2-1], 32)
			if err != nil {
				fmt.Println(err)
			}
			num2, err := strconv.ParseFloat(temp[j*2], 32)
			if err != nil {
				fmt.Println(err)
			}
			r[j-1] = recta{float32(num1), float32(num2)}
		}
		tiempos[temp[0]] = &timeProcess{r[0], r[1], r[2]}
	}
	return
}
func (n nodo) String() string {
	return fmt.Sprintf("Direccion IP: "+n.dirIP+"\nTipo: "+n.tipo+"\nSlots: %d\nSlots ocupados: %d\ntslot: "+n.tslot.String()+"\n", n.slots, n.ocupados)
}

func (c chunk) Close() {
	c.f.Close()
	err := os.Remove(c.nombre)
	if err != nil {
		fmt.Println("AAAH La puta madre:", err)
	}
}

func (c cabecera) String() string {
	return fmt.Sprint("flag: ", c.flag, "\nSorQ: ", c.sizeORqueue, "\norigen: ", c.origen, "\nTdl: ", c.Tdl.String(), "\nTrace:", c.trace)
}

/*
	Trocea un video en len(dur)-1 partes.
	El primer valor ha de ser 0 y el ultimo ha de ser igual a la duracion total del video.
*/
func cut_video(name string, dur []int) (out []string) {
	for i := 1; i < len(dur); i++ {
		temp := fmt.Sprint(i) + "-" + name
		cmd1 := exec.Command("ffmpeg", "-i", name, "-ss", fmt.Sprint(dur[i-1]), "-t", fmt.Sprint(dur[i]), "-c", "copy", "-o", temp)
		err := cmd1.Run()
		Check(err)
		out = append(out, temp)
	}
	return
}

func leerescribir(in, out *os.File) {
	b1 := make([]byte, Chunksize)
	for {
		n, e := in.Read(b1)
		if e != nil {
			return
		}
		_, err := out.Write(b1[:n])
		if err != nil {
			fmt.Println(err)
		}
	}
}
