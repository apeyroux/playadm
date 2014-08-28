package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/communaute-cimi/glay"
	//	"github.com/communaute-cimi/glay/utils"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

type Configuration struct {
	MaxFailure   int                `json:"maxfailure"`
	Applications []glay.Application `json:"apps"`
}

const VERSION = "1.0"

var (
	flid    *int  = flag.Int("id", 0, "id app. Id 0 is all apps. Use it with start/stop/restart")
	flall   *bool = flag.Bool("all", false, "action for all apps")
	flstart *bool = flag.Bool("start", false, "start play apps with id app. Id 0 is all apps")
	flstop  *bool = flag.Bool("stop", false, "stop play apps with id app. Id 0 is all apps")
	//	flpurge *bool = flag.Bool("purge", false, "purge failure instance")
	//	flrestart *bool   = flag.Bool("restart", false, "restart apps with id app. Id 0 is all apps")
	fllist    *bool   = flag.Bool("list", false, "list play apps aviable on this server")
	flnagios  *bool   = flag.Bool("nagios", false, "Nagios plugin")
	fllogs    *bool   = flag.Bool("logs", false, "View log app")
	flconfig  *string = flag.String("c", "/etc/playadm.json", "Config file")
	flversion *bool   = flag.Bool("version", true, "Show version")
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
	header := fmt.Sprintf("| %3s | %-50s | %-10s | %s |", "ID", "App", "Status", "Port")
	fmt.Printf("%s\n", strings.Repeat("-", len(header)))
	fmt.Printf("%s\n", header)
	fmt.Printf("%s\n", strings.Repeat("-", len(header)))
	for i, app := range config.Applications {
		status, _ := app.State()
		switch status {
		case glay.UP:
			port, _ := app.ListenPort()
			fmt.Printf("| %3d | %-50s | %-10s | %-4d |\n", i+1, app.Name, "Up", port)
		case glay.DOWN:
			port, _ := app.ListenPort()
			fmt.Printf("| %3d | %-50s | %-10s | %-4d |\n", i+1, app.Name, "Down", port)
		case glay.FAILURE:
			port, _ := app.ListenPort()
			fmt.Printf("| %3d | %-50s | %-10s | %-4d |\n", i+1, app.Name, "Failure", port)
		default:
			port, _ := app.ListenPort()
			fmt.Printf("| %3d | %-50s | %-10s | %-4d |\n", i+1, app.Name, "Failure", port)
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

func showlogs(config Configuration, app glay.Application) (err error) {
	log := fmt.Sprintf("%s/logs/system.out", app.Home)
	flog, err := ioutil.ReadFile(log)
	fmt.Printf("%s", string(flog))
	return
}

func main() {
	flag.Parse()

	if len(os.Args) == 1 {
		flag.Usage()
	}

	configuration, err := getConfiguration(*flconfig)

	if err != nil {
		log.Fatal("%s", err)
	}

	if *flversion {
		fmt.Printf("playadm version : %s\n", VERSION)
		fmt.Printf("glay version : %s\n", VERSION)
		os.Exit(0)
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

	if *flstop && *flid != 0 {
		app, err := getAppById(*flid-1, configuration)
		if err != nil {
			log.Fatalf("%s", err)
		}
		stop(app)
	} else if *flstop && *flall {
		stopall(configuration)
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
}
