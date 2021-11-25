package main

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

/* Variables globales */
const (
	Chunksize int     = 20000000 //Tamaño fijo
	eps       float64 = 0.5      //Tolerancia
	filePort  string  = ":9544"  //Puerto para la transferencia de archivos
)

type cabecera struct {
	flag        string    //Info, file, get_info
	sizeORqueue int64     //Tamaño del archivo o tamaño de cola
	origen      string    //Qué video es
	Tdl         time.Time //Deadline
	tpiece      float32   //Duracion de la parte
	trace       string
}

/*
	Manda la infomarcion propia del nodo al nodo de medidas.
	Como manda datos concretos no hace falta mandar una
	estructura tipo cabecera.
*/

/*
Establece una conexión TCP con el servidor de medidas
y le pide la ocupacion de un nodo en concreto.

Un nodo nunca debe hacer get_info(m["me"]), ya que estaria sobreescribiendo
su propio valor de ocupacion. De toda formas da error porque un nodo se tiene
a si mismo como "me", no como dirIP.
*/

func send_state(con net.Conn, cab cabecera) {
	enc := gob.NewEncoder(con)
	err := enc.Encode(cab)
	if err != nil {
		fmt.Println(err)
	}
}
func recive_state(rdr *bufio.Reader) (cab cabecera) {
	dec := gob.NewDecoder(rdr)
	dec.Decode(&cab)
	return
}

type info_message struct {
	valor int
	tag   string
	hora  time.Time
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
	// A simple encoding: plain text.
	b := bytes.NewBuffer(data)
	_, err := fmt.Fscanln(b, &c.flag, &c.sizeORqueue, &c.origen, &c.tpiece, &c.trace)
	if err == nil {
		c.Tdl.GobDecode(b.Bytes())
	}
	return err
}

//Todos los tiempos en milisegundos
/*
El tiempo de procesado en el servidor no se usa, ya que
segun esta ahora se procesa en el servidor como ultimo
recurso si no da tiempo a hacerlo en local o nodo intermedio
*/

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
	n = &nodo{temp[0], temp[3], temp[1], i, time.Now(), 20000000, time.Now().Local(), 0, 0}
	return
}
func auto_config() (nodos map[string]*nodo) {
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
			//infoServer = temp.dirIP
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
	return
}
func (n nodo) String() string {
	return fmt.Sprintf("Direccion IP: "+n.dirIP+"\nTipo: "+n.tipo+"\nSlots: %d\nSlots ocupados: %d\n", n.slots, n.ocupados)
}

func Check(err error) {
	if err != nil {
		panic(err)
	}
}

func (c cabecera) String() string {
	return fmt.Sprint("flag: ", c.flag, "\nSorQ: ", c.sizeORqueue, "\norigen: ", c.origen, "\nTdl: ", c.Tdl.String(), "\nTrace:", c.trace)
}
