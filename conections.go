package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var sync_register chan string = make(chan string, 1)

func Server_tcp(f func(net.Conn)) {
	port := strings.Split(m["me"].dirIP, ":")[1]
	dstream, err := net.Listen("tcp", ":"+port)
	Check(err)

	defer dstream.Close()
	for {
		con, err := dstream.Accept()
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println("Coexsión aceptada.")
			go f(con)
		}
	}
}

func Add_to_register(cab *cabecera) (id int) {
	id = rand.Int()
	sync_register <- "pillo"
	_, ok := registro[id]
	for ok {
		id = rand.Int()
		_, ok = registro[id]
	}
	registro[id] = cab
	<-sync_register
	return
}

func Retract_from_register(id int) {
	if _, ok := registro[id]; ok {
		delete(registro, id)
	}
}

func handle(con net.Conn) {
	rdr := bufio.NewReader(con)
	cab := recive_state(rdr)
	id := Add_to_register(&cab)
	fmt.Println("Tipo de cabecera:", cab.flag, "\nOrigen:", cab.origen)
	switch cab.flag {
	case "file":
		fmt.Println("Recibo cabecera de archivo:\n", cab)
		handle_file_con(id, rdr, con)
	case "get_BW":
		fmt.Println("Recibo cabecera de archivo:\n", cab)
		target := m[cab.origen]
		getBW(target)
		con.Write([]byte(fmt.Sprint(target.bandwidth)))
	case "BW":
		con.Close()
		m[cab.origen].bandwidth = int(cab.sizeORqueue)
		m[cab.origen].tbandwidt = cab.Tdl
	case "get_Q":
		send_info()
	}
}

func handle_file_con(id int, rdr *bufio.Reader, con net.Conn) {
	defer con.Close()

	b1 := make([]byte, Chunksize)
	msg, err := rdr.ReadString('\n')
	if err != nil {
		fmt.Println("Conexión cerrada por el otro extremo durante la recepción de video:\n", err)
		return
	}

	fmt.Println("Recibo cabecera del chunk: ", msg)
	msg = msg[:len(msg)-1]
	vmsg := strings.Split(msg, " ") //Tres partes, 1. Nombre 2. parte 3. Total de partes
	dir := vmsg[0] + "dir"
	os.Mkdir(dir, 0755)
	ruta := "./" + dir + "/" + vmsg[1] + "-" + fmt.Sprint(id)
	f, err := os.Create(ruta)
	Check(err)
	entero, _ := strconv.Atoi(vmsg[2])
	ch := make(chan *chunk, entero)
	//go resend(vmsg, ch, id)
	go decision(id, &ch) //¿Qué podría salir mal?
	defer close(ch)
	cuenta := 0
	for {
		n, err := rdr.Read(b1[:Chunksize-cuenta]) //Controlamos no leer mas de un chunk
		if err != nil {
			if cuenta > 0 { //En caso de que la lectura de un chunk este sin terminar
				entero, _ := strconv.Atoi(vmsg[2])
				part, _ := strconv.Atoi(vmsg[1])
				temp := &chunk{ruta, part, entero, f}
				ch <- temp //Cierro el chunk y lo mando por ch aunque este sin terminar
			}
			if err == io.EOF {
				fmt.Println(time.Now().String()[11:19] + ": Conexion cerrada por el otro extremo.")
				return
			} else {
				fmt.Println(err)
				return
			}
		}
		f.Write(b1[:n])
		cuenta += n
		if cuenta >= Chunksize { //Renovamos cuenta, cerramos fichero y creamos uno nuevo
			cuenta = 0
			entero, _ := strconv.Atoi(vmsg[2])
			part, _ := strconv.Atoi(vmsg[1])
			//temp := &chunk{vmsg[0], part, entero, f}
			temp := &chunk{ruta, part, entero, f}
			ch <- temp
			msg, err = rdr.ReadString('\n')
			Check(err)
			msg = msg[:len(msg)-1]
			vmsg = strings.Split(msg, " ")
			os.Mkdir(vmsg[0]+"dir", 0755)
			ruta = "./" + dir + "/" + vmsg[1] + "-" + fmt.Sprint(id)
			f, err = os.Create(ruta)
			fmt.Println("[CREADO]:", ruta+"\n")
			Check(err)
			fmt.Println("Recibo cabecera del chunk: ", msg)
		}
	}
}

func Enviar(nombre, tag string, cab cabecera) {
	//Dimensionar numero de chunks
	f, e := os.Open(nombre)
	//fmt.Println("Nombre del archivo:", nombre)

	Check(e)
	total, e := f.Seek(0, 2)
	Check(e)
	parteT := math.Ceil(float64(total) / float64(Chunksize))
	f.Seek(0, 0)
	parte := 0

	//Realizar Conexion
	tcpAddr, err := net.ResolveTCPAddr("tcp", m[tag].dirIP)
	Check(err)
	con, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		fmt.Println("[ERROR]: No se pudo realizar la conexcion con:", m[tag].dirIP)
		return
	}
	ttt := strings.Split(nombre, "/")
	stemp := ttt[len(ttt)-1]
	cab.trace += tag + "!"
	send_state(con, cab)
	texto := fmt.Sprint(stemp, " ", parte, " ", parteT, "\n")
	//fmt.Println("Le mando cabecera de chunk:\n", texto)
	con.Write([]byte(texto))

	b1 := make([]byte, Chunksize)
	cuenta := 0
	defer con.Close()
	for {
		n, e := f.Read(b1[:Chunksize-cuenta])

		if e != nil {
			fmt.Println(e)
			return
		}
		_, err = con.Write(b1[:n])
		if err != nil {
			fmt.Println("ERROR al escribir en archivo.\n", err)
		}
		cuenta += n
		if cuenta >= Chunksize {
			cuenta = 0
			parte++
			texto := fmt.Sprint(stemp, " ", parte, " ", parteT, "\n")
			fmt.Println(texto)
			con.Write([]byte(texto))
		}
	}
}

func getParameters() (name string, duracion, dl, repeat int64) {
	by, err := ioutil.ReadFile("streamingInfo")
	if err != nil {
		panic(err)
	}
	ss := strings.ReplaceAll(string(by), "\n", "")
	s := strings.Split(ss, " ")
	name = s[0]
	duracion, _ = strconv.ParseInt(s[1], 10, 32)
	dl, _ = strconv.ParseInt(s[2], 10, 32)
	repeat, _ = strconv.ParseInt(s[3], 10, 32)
	return
}

func genTrafic() {

	name, duracion, dl, repeat := getParameters()

	fi, err := os.Stat(name)
	if err != nil {
		panic(err)
	}
	fmt.Println("+++++++++++++++++++++++++++++\nResumen:\nDL de:", dl, "Segundos.\nPeriodicidad:", repeat, "segundos.\nVideo:", name, "\nDuracion:", duracion)
	t := time.NewTicker(time.Duration(repeat) * time.Second)
	for {
		cab := cabecera{"file", fi.Size(), name, time.Now().Local().Add(time.Duration(dl) * time.Second), float32(duracion), m["me"].tag + "!"}
		id := Add_to_register(&cab)
		ch := make(chan *chunk)
		decision(id, &ch)
		close(ch)
		<-t.C
	}
}

func getBW(v *nodo) {
	output, err := exec.Command("iperf", "-c", v.dirIP[:len(v.dirIP)-5]).Output()
	Check(err)
	vec := strings.Split(string(output), "\n")
	if len(vec) <= 6 {
		fmt.Println("Error midiendo el ancho de banda para:", v.dirIP)
		return
	}
	vec = strings.Split(vec[6], " ")

	result := vec[10:]
	fmt.Println("Resultado de medir el ancho de banda (string):", result)
	var sal float64
	for i := 0; i < len(result); i++ {
		if result[i] == "" || result[i] == " " {
			result = append(result[:i], result[i+1:]...)
			i--
		}
	}
	sal, err = strconv.ParseFloat(result[0], 32)
	fmt.Println("Ancho de banda: ", sal)
	Check(err)
	if result[1] == "Gbits/sec" {
		sal *= 1000000000
	} else if result[1] == "Mbits/sec" {
		sal *= 1000000
	} else if result[1] == "Kbits/sec" {
		sal *= 1000
	}
	v.bandwidth = int(sal)
	v.tbandwidt = time.Now().Local()
}
