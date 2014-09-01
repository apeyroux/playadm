package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/communaute-cimi/glay"
	"github.com/communaute-cimi/linuxproc"
	//	"github.com/communaute-cimi/glay/utils"
	"code.google.com/p/go.net/websocket"
	"errors"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type Configuration struct {
	MaxFailure   int                `json:"maxfailure"`
	Applications []glay.Application `json:"apps"`
}

const VERSION = "1.0.2"

var (
	flid    *int  = flag.Int("id", 0, "id app. Id 0 is all apps. Use it with start/stop/restart")
	flall   *bool = flag.Bool("all", false, "action for all apps")
	flstart *bool = flag.Bool("start", false, "start play apps with id app. Id 0 is all apps")
	flstop  *bool = flag.Bool("stop", false, "stop play apps with id app. Id 0 is all apps")
	flclean *bool = flag.Bool("clean", false, "clean failure instance")
	//	flrestart *bool   = flag.Bool("restart", false, "restart apps with id app. Id 0 is all apps")
	fllist    *bool   = flag.Bool("list", false, "list play apps aviable on this server")
	flnagios  *bool   = flag.Bool("nagios", false, "Nagios plugin")
	fllogs    *bool   = flag.Bool("logs", false, "View log app")
	flconfig  *string = flag.String("c", "/etc/playadm.json", "Config file")
	flversion *bool   = flag.Bool("version", false, "Show version")
	flhttpd   *bool   = flag.Bool("httpd", false, "httpd")
	fllisten  *string = flag.String("listen", ":8080", "Listen ip:port or :port")
)

func getConfiguration(configpath string) (configuration Configuration, err error) {
	configuration = Configuration{}
	fconfig, err := ioutil.ReadFile(configpath)
	if err != nil {
		return
	}

	err = json.Unmarshal(fconfig, &configuration)

	if err != nil {
		return
	}

	return
}

func listApps(config Configuration) {
	header := fmt.Sprintf("| %3s | %-50s | %-10s | %-4s | %-12s |", "ID", "App", "Status", "Port", "VmData")
	fmt.Printf("%s\n", strings.Repeat("-", len(header)))
	fmt.Printf("%s\n", header)
	fmt.Printf("%s\n", strings.Repeat("-", len(header)))
	for i, app := range config.Applications {
		status, _ := app.State()
		pid, _ := app.Pid()
		proc, _ := linuxproc.FindProcess(pid)
		vmdata, _ := proc.VmData()
		switch status {
		case glay.UP:
			port, _ := app.ListenPort()
			fmt.Printf("| %3d | %-50s | %-10s | %-4d | %-12s |\n", i+1, app.Name, "Up", port, vmdata)
		case glay.DOWN:
			port, _ := app.ListenPort()
			fmt.Printf("| %3d | %-50s | %-10s | %-4d | %-12s |\n", i+1, app.Name, "Down", port, vmdata)
		case glay.FAILURE:
			port, _ := app.ListenPort()
			fmt.Printf("| %3d | %-50s | %-10s | %-4d | %-12s |\n", i+1, app.Name, "Failure", port, vmdata)
		default:
			port, _ := app.ListenPort()
			fmt.Printf("| %3d | %-50s | %-10s | %-4d | %-12s |\n", i+1, app.Name, "Failure", port, vmdata)
		}
	}
	fmt.Printf("%s\n", strings.Repeat("-", len(header)))
	return
}

func getAppById(id int, configuration Configuration) (app glay.Application, err error) {
	if len(configuration.Applications)-1 < id {
		return app, errors.New("Your app is not in config file.")
	}

	app = configuration.Applications[id]

	return
}

func startall(configuration Configuration) {
	for _, app := range configuration.Applications {
		start(app)
	}
}

func start(app glay.Application) {
	execr := app.Start()
	if execr.Err != nil {
		log.Printf("%s - %s", app.Name, execr.Err)
	} else {
		log.Printf("%s - \n%s", app.Name, execr.Output)
	}
}

func stopall(configuration Configuration) {
	for _, app := range configuration.Applications {
		stop(app)
	}
}

func stop(app glay.Application) {
	execr := app.Stop()
	if execr.Err != nil {
		log.Printf("%s - %s", app.Name, execr.Err)
	} else {
		log.Printf("%s - \n%s", app.Name, execr.Output)
	}
}

func cleanall(configuration Configuration) {
	for _, app := range configuration.Applications {
		clean(app)
	}
}

func clean(app glay.Application) {
	err := app.Clean()
	if err != nil {
		log.Printf("%s - %s", app.Name, err)
	} else {
		log.Printf("%s - is clean", app.Name)
	}
}

func showlogs(config Configuration, app glay.Application) (err error) {
	log := fmt.Sprintf("%s/logs/system.out", app.Home)
	flog, err := ioutil.ReadFile(log)
	fmt.Printf("%s", string(flog))
	return
}

func mainHandler(configuration Configuration) http.Handler {

	type App struct {
		Name   string
		Pid    int
		VmData int
		Port   int
		State  glay.State
	}

	type Data struct {
		Apps     []App
		MemTotal float32
		MemFree  float32
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tpl, err := template.ParseFiles("tpl/main.html")
		if err != nil {
			log.Printf("%s", err)
		}

		data := new(Data)

		for _, app := range configuration.Applications {
			state, _ := app.State()
			pid, _ := app.Pid()
			proc, _ := linuxproc.FindProcess(pid)
			vmdata, _ := proc.VmData()
			port, _ := app.ListenPort()
			a := App{app.Name, pid, vmdata, port, state}
			data.Apps = append(data.Apps, a)
		}

		memory := new(linuxproc.Memory)
		memFree, _ := memory.MemFree()
		data.MemFree = float32(memFree) * 9.53674316406E-7
		memTotal, _ := memory.MemTotal()
		data.MemTotal = float32(memTotal)*9.53674316406E-7 - data.MemFree

		err = tpl.Execute(w, data)
		if err != nil {
			log.Printf("%s", err)
		}
	})
}

func wsMemoryConsoHandler(configuration Configuration) func(ws *websocket.Conn) {
	return func(ws *websocket.Conn) {
		for {
			memory := new(linuxproc.Memory)
			mf, _ := memory.MemFree()
			mt, _ := memory.MemTotal()
			// utiliser json.MArshal car là, c'est moche ...
			msg := fmt.Sprintf("[{\"name\":\"free\",\"y\":%.2f,\"color\":\"#D0FA58\"},{\"name\":\"occ\",\"y\":%.2f,\"color\":\"#F78181\"}]", float32(mf)*9.53674316406E-7, float32(mt)*9.53674316406E-7-float32(mf)*9.53674316406E-7)
			name := []byte(msg)
			ws.Write(name)
			time.Sleep(250 * time.Millisecond)
		}
	}
}

func wsMemoryProcessGraphHandler(configuration Configuration) func(*websocket.Conn) {
	return func(ws *websocket.Conn) {
		i := 0
		for {
			lpoints := []string{}
			for _, a := range configuration.Applications {
				t := time.Now().Unix()
				// utiliser json.MArshal car là, c'est moche ...
				pid, _ := a.Pid()
				p, _ := linuxproc.FindProcess(pid)
				vmpeak, _ := p.VmPeak()
				lpoints = append(lpoints, fmt.Sprintf("{\"x\":%d,\"y\":%.2f,\"name\":\"%s\"}", t, float32(vmpeak)*9.53674316406E-7, a.Name))
			}
			msg, _ := json.Marshal(lpoints)
			ws.Write(msg)
			time.Sleep(2 * time.Second)
			i += 1
		}
	}
}

func wsMemoryProcessHandler(configuration Configuration) func(*websocket.Conn) {
	return func(ws *websocket.Conn) {
		i := 0
		for {
			apps := configuration.Applications
			appspoints := []string{}
			for _, a := range apps {
				// utiliser json.MArshal car là, c'est moche ...
				pid, _ := a.Pid()
				p, _ := linuxproc.FindProcess(pid)
				vmdata, _ := p.VmData()
				vmpeak, _ := p.VmPeak()
				state, _ := a.State()
				appspoints = append(appspoints, fmt.Sprintf("{\"peak\":%.2f,\"data\":%.2f,\"name\":\"%s\",\"pid\":%d,\"state\":\"%d\"}", float32(vmdata)*9.53674316406E-7, float32(vmpeak)*9.53674316406E-7, a.Name, pid, state))
			}
			msg, _ := json.Marshal(appspoints)
			ws.Write(msg)
			time.Sleep(2000 * time.Millisecond)
			i += 1
		}
	}
}

func main() {
	flag.Parse()

	if len(os.Args) == 1 {
		flag.Usage()
	}

	configuration, err := getConfiguration(*flconfig)

	if err != nil {
		log.Fatalf("%s", err)
	}

	if *flversion {
		fmt.Printf("playadm version : %s\n", VERSION)
		fmt.Printf("glay version : %s\n", glay.VERSION)
		os.Exit(0)
	}

	if *flclean && *flid != 0 {
		app, err := getAppById(*flid-1, configuration)
		if err != nil {
			log.Fatalf("%s", err)
		}
		clean(app)
	} else if *flclean && *flall {
		cleanall(configuration)
	}

	if *flstop && *flid != 0 {
		app, err := getAppById(*flid-1, configuration)
		if err != nil {
			log.Fatalf("%s", err)
		}
		stop(app)
	} else if *flstop && *flall {
		stopall(configuration)
	}

	if *flstart && *flid != 0 {
		app, err := getAppById(*flid-1, configuration)
		if err != nil {
			log.Fatalf("%s", err)
		}
		start(app)
	} else if *flstart && *flall {
		startall(configuration)
	}

	if *flnagios {
		glay.NagiosPlugin(configuration.MaxFailure, configuration.Applications)
	}

	if *fllogs && *flid != 0 {
		app, err := getAppById(*flid-1, configuration)
		if err != nil {
			log.Fatalf("%s", err)
		}
		err = showlogs(configuration, app)
		if err != nil {
			log.Printf("%s", err)
		}
	}

	if *fllist {
		listApps(configuration)
	}

	if *flhttpd {
		// http://stackoverflow.com/questions/17541333/fileserver-handler-with-some-other-http-handlers
		http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
		http.Handle("/wsmemoryconsograph/", websocket.Handler(wsMemoryConsoHandler(configuration)))
		http.Handle("/wsmemoryprocessgraph/", websocket.Handler(wsMemoryProcessGraphHandler(configuration)))
		http.Handle("/wsmemoryprocess/", websocket.Handler(wsMemoryProcessHandler(configuration)))
		http.Handle("/", mainHandler(configuration))
		http.ListenAndServe(*fllisten, nil)
	}
}
