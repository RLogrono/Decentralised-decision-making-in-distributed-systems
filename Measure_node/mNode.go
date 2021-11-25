package main

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

var m map[string]*nodo
var servers, inters, pys []string

func main() {

	m = auto_config()
	infoPort := ":" + strings.Split(m["me"].dirIP, ":")[1]

	go func(port string) { //Lanza goroutine para socket UDP
		protocol := "udp"
		udpAddr, err := net.ResolveUDPAddr(protocol, port)
		if err != nil {
			fmt.Println("Wrong Address")
			return
		}
		udpCon, err := net.ListenUDP(protocol, udpAddr)
		if err != nil {
			fmt.Println(err)
		}
		for {
			handleUDP(udpCon)
		}

	}(infoPort)

	go timers()

	dstream, err := net.Listen("tcp", infoPort)
	Check(err)
	defer dstream.Close()
	for {
		con, err := dstream.Accept()
		if err != nil {
			fmt.Println(err)
		} else {
			go handleTCP(con)
		}
	}
}

func handleUDP(con *net.UDPConn) {
	var buf [512]byte
	n, _, err := con.ReadFromUDP(buf[0:])
	if err != nil {
		return
	}
	go func() {
		var im info_message
		if err = gob.NewDecoder(bytes.NewReader(buf[:n])).Decode(&im); err != nil {
			fmt.Println("Mensaje UDP no válido.\n", string(buf[:n]))
			return
		}
		if val, ok := m[im.tag]; ok {
			val.ocupados = im.valor
			val.tslot = im.hora
			fmt.Println(im)
		} else {
			fmt.Println("El cliente:", im.tag, ", no esta en la lista.\nAun asi, el mensaje es:", im)
		}
	}()
}

func (im info_message) String() string {
	text := ""
	text += fmt.Sprint(im.valor) + " " + im.tag + " " + im.hora.String()
	return text
}

func handleTCP(con net.Conn) {
	rdr := bufio.NewReader(con)
	cab := recive_state(rdr)
	fmt.Println("Recibo cabecera:", cab.flag)

	if cab.flag == "get_info" {
		enc := gob.NewEncoder(con)
		im := info_message{m[cab.origen].ocupados, "a", m[cab.origen].tslot}
		enc.Encode(im)
	} else if cab.flag == "results" {
		s, err := rdr.ReadString('\n')
		fmt.Println(s)
		if err != io.EOF || err != nil {
			Check(err)
		}

		f, err := os.OpenFile("results.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		Check(err)
		_, err = f.Write([]byte(cab.Tdl.String() + "\nFile:" + cab.origen + ";Trace:" + cab.trace + ";Diferencia:" + s))
		Check(err)
	}
}

func timers() {
	t_intermedio := time.NewTicker(5 * time.Minute)
	defer t_intermedio.Stop()
	t_servidor := time.NewTicker(12 * time.Minute)
	defer t_servidor.Stop()
	t_medidas := time.NewTicker(30 * time.Second)
	defer t_medidas.Stop()
	t_inter_server := time.NewTicker(4 * time.Minute)
	defer t_inter_server.Stop()
	for {
		select {
		case <-t_servidor.C: //NODO SERVIDOR
			fmt.Println("Aqui mediria el ancho de banda pi-server")
			fmt.Println("TIMER DE NODO SERVIDOR")
			lista := Con_checker(pys)
			if len(lista) <= 0 {
				fmt.Println("No hay ningun nodo activo")
				break
			}
			con, _ := DoCon(m[lista[0]].dirIP)
			send_state(con, cabecera{"get_BW", 0, servers[0], time.Now().Local(), 0, "a"})
			var b [50]byte //Leer entero
			n, err := con.Read(b[:])
			if err != nil {
				if err.Error() == "EOF" {
					fmt.Println("[ERROR]: El otro extremo cerro la conexion durante la medida.")
				} else {
					Check(err)
				}
			}
			m[pys[1]].bandwidth, err = strconv.Atoi(string(b[:n]))
			Check(err)
			con.Close()

			for _, v := range lista[1:] {
				con, _ = DoCon(m[v].dirIP)
				send_state(con, cabecera{"BW", int64(m[pys[1]].bandwidth), servers[0], time.Now().Local(), 0, "a"})
				con.Close()
			}
			get_Q(servers[0])
		case <-t_intermedio.C: //NODO INTERMEDIO
			fmt.Println("Aqui mediria el ancho de banda pi-node")
			fmt.Println("TIMER DE NODO INTERMDIO")
			var con *net.TCPConn
			var err error
			lista := Con_checker(pys)
			if len(lista) > 0 {
				con, _ = DoCon(m[lista[0]].dirIP)
				send_state(con, cabecera{"get_BW", 0, inters[0], time.Now().Local(), 0, "a"})
			} else {
				fmt.Println("No hay ningun nodo activo")
				break
			}

			s_dato, err := recive_BW_data(con)
			if err != nil {
				fmt.Println("[ERROR]:Conexión finalizada por el otro extremo durante la medida del ancho de banda.")
				break
			}
			m[inters[0]].bandwidth, err = strconv.Atoi(s_dato)
			Check(err)
			fmt.Println("Ancho de banda:", m[inters[0]].bandwidth)
			con.Close()

			for _, v := range lista[1:] {
				con, _ = DoCon(m[v].dirIP)
				send_state(con, cabecera{"BW", int64(m[inters[0]].bandwidth), inters[0], time.Now().Local(), 0, "a"})
				con.Close()
			}
			get_Q(inters[0])
		case <-t_medidas.C:
			Check_Qt(servers[0])
			Check_Qt(inters[0])
		case <-t_inter_server.C:
			fmt.Println("MIDO ANCHO DE BANDA inter-server")
			con, err := DoCon(m[inters[0]].dirIP)
			if err != nil {
				fmt.Println("Nodo intermedio esta inactivo.")
				break
			}
			send_state(con, cabecera{"get_BW", 0, servers[0], time.Now().Local(), 0, "a"})
			s_dato, err := recive_BW_data(con)
			if err != nil {
				fmt.Println("[ERROR]:Conexión finalizada por el otro extremo durante la medida del ancho de banda.")
				break
			}
			m[inters[0]].bandwidth, err = strconv.Atoi(s_dato)
			Check(err)
			fmt.Println("Ancho de banda:", m[inters[0]].bandwidth)
			con.Close()
		}
	}
}

func DoCon(t string) (*net.TCPConn, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", t)
	Check(err)
	con, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		fmt.Println("[ERROR]: Fallo al realizar la conexion:\n" + err.Error())
	}
	return con, err
}

func Con_checker(s []string) []string {
	var con *net.TCPConn
	var err error
	var out []string = make([]string, 0, 4)
	fmt.Println("Con_checker va a nalizar la lista:", s)
	for i := 0; i < len(s); i++ {
		con, err = DoCon(m[s[i]].dirIP)
		if err == nil {
			con.Close()
			out = append(out, s[i])
		}
	}
	return out
}

func Check_Qt(tag string) {
	if 2.0 <= time.Now().Local().Sub(m[tag].tslot).Minutes() {
		get_Q(tag)
	}
}

func get_Q(tag string) {
	con, err := DoCon(m[tag].dirIP)
	if err != nil {
		fmt.Println("[ERROR]: No se puedo realizar la conexion con", tag)
		return
	}
	send_state(con, cabecera{"get_Q", 0.0, "", time.Now(), 0, ""})
	con.Close()
}

func recive_BW_data(con net.Conn) (string, error) {
	var b [50]byte //Leer entero
	n, err := con.Read(b[:])
	if err != nil {
		if err.Error() == "EOF" {
			return "", err
		} else {
			panic(err)
		}
	}
	return string(b[:n]), nil
}
