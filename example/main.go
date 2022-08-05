package main

import (
	"flag"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	//	"net/http"
	//	_ "net/http/pprof"

	ggthx "github.com/assemblaj/ggthx/pkg"
	"github.com/hajimehoshi/ebiten/v2"
)

type peerAddress struct {
	ip   string
	port int
}

func getPeerAddress(address string) peerAddress {
	peerIPSlice := strings.Split(address, ":")
	if len(peerIPSlice) < 2 {
		panic("Please enter IP as ip:port")
	}
	peerPort, err := strconv.Atoi(peerIPSlice[1])
	if err != nil {
		panic("Please enter integer port")
	}
	return peerAddress{
		ip:   peerIPSlice[0],
		port: peerPort,
	}
}

func main() {
	/*
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()*/

	argsWithoutProg := os.Args[1:]
	if len(argsWithoutProg) < 4 {
		panic("Must enter <port> <num players> ('local' |IP adress) ('local' |IP adress) currentPlayer or <port> <num players> spectate <host ip>:<host port>")
	}
	var localPort, numPlayers int
	var err error
	localPort, err = strconv.Atoi(argsWithoutProg[0])
	if err != nil {
		panic("Plase enter integer port")
	}

	numPlayers, err = strconv.Atoi(argsWithoutProg[1])
	if err != nil {
		panic("Please enter integer numPlayers")
	}

	/*
		logFileName := ""
		if len(argsWithoutProg) > 4 {
			logFileName = "Player" + argsWithoutProg[4] + ".log"
		} else {
			logFileName = "Spectator.log"
		}

		f, err := os.OpenFile(logFileName, os.O_CREATE|os.O_RDWR, 0666)
		if err != nil {
			panic(err)
		}

		// don't forget to close it
		defer f.Close()

		ggthx.EnableLogger()
		ggthx.SetLoggerOutput(f)*/

	if argsWithoutProg[2] == "spectate" {
		hostIp := argsWithoutProg[3]
		hostAddress := getPeerAddress(hostIp)
		GameInitSpectator(localPort, numPlayers, hostAddress.ip, hostAddress.port)
	} else {
		ipAddress := []string{argsWithoutProg[2], argsWithoutProg[3]}

		currentPlayer, err = strconv.Atoi(argsWithoutProg[4])
		if err != nil {
			panic("Please enter integer currentPlayer")
		}

		players := make([]ggthx.Player, ggthx.MaxPlayers+ggthx.MaxSpectators)
		var i int
		for i = 0; i < numPlayers; i++ {
			if ipAddress[i] == "local" {
				players[i] = ggthx.NewLocalPlayer(20, i+1)
			} else {
				remoteAddress := getPeerAddress(ipAddress[i])
				players[i] = ggthx.NewRemotePlayer(20, i+1, remoteAddress.ip, remoteAddress.port)
			}
		}

		offset := 5
		numSpectators := 0
		for offset < len(argsWithoutProg) {
			remoteAddress := getPeerAddress(argsWithoutProg[offset])
			players[i] = ggthx.NewSpectator(20, remoteAddress.ip, remoteAddress.port)
			numSpectators++
			i++
			offset++
		}
		GameInit(localPort, numPlayers, players, numSpectators)
	}

	flag.Parse()

	start = int(time.Now().UnixMilli())
	now = start
	next = start
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}

}
