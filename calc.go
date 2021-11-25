package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"time"
	"unsafe"

	"github.com/keysight/clipsgo/pkg/clips"
)

var syncro chan int = make(chan int, 1)

func lineal_regresion(vid *video, tipo string) float32 {
	var out float32
	if tipo == "inter" {
		out = tprcc[vid.name].inter.y(vid.dur * vid.fps)
	} else if tipo == "pi" {
		out = tprcc[vid.name].pi.y(vid.dur * vid.fps)
	} else {
		out = tprcc[vid.name].server.y(vid.dur * vid.fps)
	}
	return out
}

/*
Tprc + Tcola + Tx > Tdl
*/

func calcTime_clips(env *clips.Environment, vid video, target string) {

	var local bool = false
	if target == "me" {
		local = true
	} else {
		get_info(target)
		fmt.Println("Pillo info de", target, "Su ocupacion es de", m[target].ocupados, "hace", m[target].tbandwidt)
	}

	//TRANSFER PARAMETERS ASSERT
	tf := "(assert (transfer-to-calc "
	tf += m["me"].tipo + " " + m[target].tipo + " " + fmt.Sprint(vid.size) + " "
	tx_cf := (10 - time.Now().Local().Sub(m[target].tbandwidt).Minutes()) / 10
	if local {
		tx_cf = 1.0
		tf += "26000000"
	} else {
		if m[target].bandwidth > 0 {
			tf += fmt.Sprint(m[target].bandwidth)
		} else {
			tf += "2000000"
		}
	}

	tf += " " + fmt.Sprint(Chunksize) + " " + fmt.Sprintf("%.2f", tx_cf) + "))"

	//WAITING PARAMETERS ASSERT
	wt := "(assert (waiting-to-calc "
	cola := m[target].ocupados - m[target].slots + 1
	if cola < 0 {
		cola = 0
	}
	wt += m[target].tipo + " " + fmt.Sprint(cola) + " "
	tmedio := 25.0
	if m[target].tipo == "inter" {
		tmedio = 127.0
	}
	wt += fmt.Sprintf("%.1f", tmedio) + " "
	var cf_w float64 = 1.0
	if !local { //Si no es en local entonces:
		dif := time.Now().Local().Sub(m[target].tslot).Seconds()
		cf_w = 1 - (dif*0.2)/float64(tmedio)
		fmt.Println("CF:", cf_w, "\nDiferencia:", dif)
	}
	wt += fmt.Sprintf("%.2f", cf_w) + "))"

	//EXEC PARAMETERS ASSERT
	et := "(assert (exec-to-calc " + m[target].tipo + " "
	frames := int(vid.dur * vid.fps)
	et += fmt.Sprint(frames) + " "
	var mp, n float32
	if m[target].tipo == "inter" {
		mp = tprcc[vid.name].inter.m
		n = tprcc[vid.name].inter.n
	} else if m[target].tipo == "pi" {
		mp = tprcc[vid.name].pi.m
		n = tprcc[vid.name].pi.n
	} else {
		mp = tprcc[vid.name].server.m
		n = tprcc[vid.name].server.n
	}
	et += fmt.Sprint(mp) + " " + fmt.Sprint(n) + "))"

	env.SendCommand(tf)
	env.SendCommand(wt)
	env.SendCommand(et)
}

func calcTime(vid video, target string, Tdl time.Time) (out float64) {

	var temptipo string = m[target].tipo
	var T float32 = 0.0
	var total float32 = 0

	//tx
	if temptipo == "me" {
		T = 0
	} else {
		if m[target].bandwidth <= 0 {
			T = 0
			fmt.Println("[ERROR]: bandwidth <= 0")
		} else {
			T = float32((vid.size + 41*((vid.size)/Chunksize)) / m[target].bandwidth)
		}
	}
	total += T
	//tprc
	T = lineal_regresion(&vid, temptipo)
	total += T

	//tcola
	if m[target].ocupados >= m[target].slots {
		qsize := m[target].ocupados - m[target].slots
		if m[target].tipo == "server" {
			T = float32((qsize + 1) * 25) //El 25 es la media de tiempo de espera de todos los videos con ocupacion de 5, dividida entre 10 (5 * 2)
		} else if m[target].tipo == "inter" {
			T = float32((qsize + 1) * 127) //Lo mismo para el nodo intermedio
		}
	} else {
		T = 0
	}
	total += T

	calc := time.Now().Local().Add(time.Duration(total) * time.Second) //resta de tiempo
	return Tdl.Sub(calc).Seconds()
}

func actionWhenProcessingAtMacmini(s_ptr string, id int) {
	fmt.Println("Execution to be accomplished at the macmini")
	ch := get_chan(s_ptr)
	if m["me"].tipo == "pi" {
		cab := registro[id]
		Enviar(cab.origen, inters[0], *cab)
	} else {
		send_while_reciving(*ch, id, inters[0])
	}
}
func actionWhenProcessingAtServer(s_ptr string, id int) {
	fmt.Println("Execution to be accomplished at the server")
	ch := get_chan(s_ptr)
	if m["me"].tipo == "pi" {
		cab := registro[id]
		Enviar(cab.origen, servers[0], *cab)
	} else {
		send_while_reciving(*ch, id, servers[0])
	}
}
func actionWhenProcessingAtLocal(s_ptr string, id int) {
	fmt.Println("Execution to be accomplished at the local")
	ch := get_chan(s_ptr)
	name := build_file(*ch, id)
	Prcss_video(name, id)
}
func actionWhenNoAction(s_ptr string, id int, place string) {
	switch place {
	case "inter":
		if m["me"].tipo == "inter" {
			actionWhenProcessingAtLocal(s_ptr, id)
		} else {
			actionWhenProcessingAtMacmini(s_ptr, id)
		}
	case "server":
		if m["me"].tipo == "server" {
			actionWhenProcessingAtLocal(s_ptr, id)
		} else {
			actionWhenProcessingAtServer(s_ptr, id)
		}

	case "local":
		actionWhenProcessingAtLocal(s_ptr, id)
	}
}

func get_chan(s string) (ptr *chan *chunk) {
	adr, err := strconv.ParseUint(s, 0, 64)
	if err != nil {
		panic(err)
	}
	var temp_ptr uintptr = uintptr(adr)
	unsafe_ptr := unsafe.Pointer(temp_ptr)
	ptr = (*chan *chunk)(unsafe_ptr)
	return
}

/*
Crea un entorno de clips y carga ahi todos los parametros para que decida donde enviar.
No devuelve ningun valor porque usa la funcion de callback para ejecutar la orden.
*/

func clips_decision(id int, ch *chan *chunk) {
	cab := registro[id]
	fps := fps_elector(cab.origen)
	var vid video = video{cab.origen, int(cab.sizeORqueue), cab.tpiece, fps}

	env := clips.CreateEnvironment()

	defer env.Delete()

	//Establezce las posibles acciones que puede tomar el entorno-clips
	callback := actionWhenProcessingAtMacmini
	err := env.DefineFunction("process-at-macmini", callback)
	if err != nil {
		fmt.Println("Error with the callback declaration1")
	}
	callback = actionWhenProcessingAtServer
	err = env.DefineFunction("process-at-server", callback)
	if err != nil {
		fmt.Println("Error with the callback declaration2")
	}
	callback = actionWhenProcessingAtLocal
	err = env.DefineFunction("process-at-local", callback)
	if err != nil {
		fmt.Println("Error with the callback declaration3")
	}
	callback2 := actionWhenNoAction
	err = env.DefineFunction("no-action", callback2)
	if err != nil {
		fmt.Println("Error with the callback declaration4")
	}

	//Configuración del en entorno-clips
	env.SendCommand("(set-salience-evaluation when-activated)")
	env.Load("data/decision-spec-rules.clp")
	rules := env.Rules()
	fmt.Println("Found ", len(rules), " rules in the system")

	switch m["me"].tipo {
	case "pi":
		calcTime_clips(env, vid, servers[0])
		calcTime_clips(env, vid, inters[0])
	case "inter":
		calcTime_clips(env, vid, servers[0])
		calcTime_clips(env, vid, "me")
	case "server":
		calcTime_clips(env, vid, "me")
	}
	env.SendCommand("(facts)")
	fmt.Println("******** SEPARADOR *********")
	diff := cab.Tdl.Sub(time.Now().Local())
	seconds := diff.Seconds()
	env.SendCommand("(assert (deadline " + fmt.Sprint(seconds) + "))")
	env.SendCommand("(assert (pointer chunk-chan " + fmt.Sprint(ch) + " " + fmt.Sprint(id) + "))")
	env.SendCommand("(run)")
	env.SendCommand("(facts)")
	env.SendCommand("(agenda)")
	env.SendCommand("(assert (fire executionr))")
	env.SendCommand("(run)")
}

func decision(id int, ch *chan *chunk) {
	if rules {
		clips_decision(id, ch)
	} else {
		IFTHEN_decision(id, ch)
	}
}

func IFTHEN_decision(id int, ch *chan *chunk) {
	dec, dest := post_decision(registro[id])
	if dec == 0 {
		if dest == servers[0] {
			actionWhenProcessingAtServer(fmt.Sprint(ch), id)
		} else {
			actionWhenProcessingAtMacmini(fmt.Sprint(ch), id)
		}
	} else {
		actionWhenProcessingAtLocal(fmt.Sprint(ch), id)
	}
}

/*
out = 0 reenviar;
out = 1 guardar;
out = 2 procesar;
*/
func post_decision(cab *cabecera) (out int, dest string) {
	var temptipo string = m["me"].tipo
	fps := fps_elector(cab.origen)
	var vid video = video{cab.origen, int(cab.sizeORqueue), cab.tpiece, fps}

	if m["me"].tipo == "server" {
		fmt.Println("Decision: proceso local; motivo: soy servidor")
		return 2, ""
	}
	if m["me"].ocupados < m["me"].slots { //Proceso en local
		if calcTime(vid, "me", cab.Tdl) > 0+eps {
			fmt.Println("Decision: proceso local; motivo: da tiempo")
			return 2, ""
		}
	} /* Puede ser que no de tiempo por poco y que en el server vaya a tardar mas */
	if temptipo == "inter" {
		dest, err := buscar_maq("server")
		if err != nil {
			fmt.Println("Decision: proceso local; motivo: error al buscar servidor")
			return 2, ""
		} else {
			fmt.Println("Decision: envia a server; motivo: inter ocupado")
			return 0, dest
		}
	} else { /* pinode decide a quien mandar, en local no le da tiempo */
		dest, err := buscar_maq("inter")
		if err != nil {
			dest, _ = buscar_maq("server")
			fmt.Println("Decision: envia a server; motivo: error al buscar inter")
			return 0, dest
		}
		T := calcTime(vid, dest, cab.Tdl)
		if T > 0+eps { //Mandar a nodo Intermedio
			fmt.Println("Decision: envia intermedio; motivo: le da tiempo (tiempo: ", T, ")")
			return 0, dest
		} else {
			dest, _ := buscar_maq("server")
			fmt.Println("Decision: envia a server; motivo: falta timepo (timepo: ", T, ")")
			return 0, dest
		}
	}
}

func fps_elector(s string) (fps float32) {
	switch s {
	case "alex.mp4":
		fps = 10
	case "240p1.mp4":
		fps = 24
	case "trafico.mp4":
		fps = 23.98
	case "puertaSol.mp4":
		fps = 25
	case "Interseccion.mp4":
		fps = 25
	default:
		fps = 24.0
	}
	return
}

func Prcss_video(name string, id int) {
	cab := registro[id]
	syncro <- 1
	m["me"].ocupados++
	<-syncro
	send_info()
	pcCola <- "entro"

	cmd := exec.Command("python3", "run.py", name)
	outfile, err := os.Create("./" + name[:len(name)-4] + ".txt")
	Check(err)
	defer outfile.Close()
	cmd.Stdout = outfile
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Println("[ERROR]: Error al procesar\n", err)
		tcpAddr, err := net.ResolveTCPAddr("tcp", infoServer)
		Check(err)
		con, err := net.DialTCP("tcp", nil, tcpAddr)
		Check(err)
		send_state(con, cabecera{"results", 0, cab.origen, time.Now().Local(), 0, cab.trace})
		_, err = con.Write([]byte("ERROR AL PROCESAR\n"))
		if err != nil {
			fmt.Println("[ERROR]: Error al mandar los resultados.")
		}
		con.Close()
	} else {
		go send_result(cab)
	}

	syncro <- 1
	m["me"].ocupados--
	<-syncro
	go send_info()
	<-pcCola
	fmt.Println("¡He terminado de procesar: ", name, "!")
	Retract_from_register(id)
}

func buscar_maq(s string) (out string, err error) { // Esta funcion tiene que desaparecer
	fmt.Println("Busco máquina:", s)
	if s == "server" {
		var temp *nodo
		for i := 0; i < len(servers); i++ {
			//get_info(servers[i])
			temp = m[servers[i]]
			return temp.tag, nil
		}
		//return "", decisionErr{"No se han encontrado servers"}
		return servers[0], nil
	} else if s == "inter" {
		fmt.Println(inters)
		var temp *nodo
		for i := 0; i < len(inters); i++ {
			get_info(inters[i])
			temp = m[inters[i]]
			return temp.tag, nil
		}
		return "", io.EOF
	}
	return "", io.EOF
}
