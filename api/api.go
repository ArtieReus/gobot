package api

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/bmizerany/pat"
	"github.com/hybridgroup/gobot"
	"github.com/hybridgroup/gobot/api/robeaux"
)

// Optional restful API through Gobot has access
// all the robots.
type api struct {
	gobot    *gobot.Gobot
	router   *pat.PatternServeMux
	Host     string
	Port     string
	Username string
	Password string
	Cert     string
	Key      string
	handlers []func(http.ResponseWriter, *http.Request)
	start    func(*api)
}

func NewAPI(g *gobot.Gobot) *api {
	return &api{
		gobot: g,
		Port:  "3000",
		start: func(a *api) {
			log.Println("Initializing API on " + a.Host + ":" + a.Port + "...")
			http.Handle("/", a)

			go func() {
				if a.Cert != "" && a.Key != "" {
					http.ListenAndServeTLS(a.Host+":"+a.Port, a.Cert, a.Key, nil)
				} else {
					log.Println("WARNING: API using insecure connection. " +
						"We recommend using an SSL certificate with Gobot.")
					http.ListenAndServe(a.Host+":"+a.Port, nil)
				}
			}()
		},
	}
}

func (a *api) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	for _, handler := range a.handlers {
		handler(res, req)
	}
	a.router.ServeHTTP(res, req)
}

func (a *api) Post(path string, f func(http.ResponseWriter, *http.Request)) {
	a.router.Post(path, http.HandlerFunc(f))
}

func (a *api) Put(path string, f func(http.ResponseWriter, *http.Request)) {
	a.router.Put(path, http.HandlerFunc(f))
}

func (a *api) Delete(path string, f func(http.ResponseWriter, *http.Request)) {
	a.router.Del(path, http.HandlerFunc(f))
}

func (a *api) Options(path string, f func(http.ResponseWriter, *http.Request)) {
	a.router.Options(path, http.HandlerFunc(f))
}

func (a *api) Get(path string, f func(http.ResponseWriter, *http.Request)) {
	a.router.Get(path, http.HandlerFunc(f))
}

func (a *api) Head(path string, f func(http.ResponseWriter, *http.Request)) {
	a.router.Head(path, http.HandlerFunc(f))
}

func (a *api) AddHandler(f func(http.ResponseWriter, *http.Request)) {
	a.handlers = append(a.handlers, f)
}

func (a *api) SetBasicAuth(user, password string) {
	a.Username = user
	a.Password = password
	a.AddHandler(a.basicAuth)
}

func (a *api) SetDebug() {
	a.AddHandler(func(res http.ResponseWriter, req *http.Request) {
		log.Println(req)
	})
}

// start starts the api using the start function
// sets on the API on initialization.
func (a *api) Start() {
	a.router = pat.New()

	mcpCommandRoute := "/commands/:command"
	deviceCommandRoute := "/robots/:robot/devices/:device/commands/:command"
	robotCommandRoute := "/robots/:robot/commands/:command"

	// api
	a.Get("/", a.mcp)
	a.Get("/commands", a.mcpCommands)
	a.Get(mcpCommandRoute, a.executeMcpCommand)
	a.Post(mcpCommandRoute, a.executeMcpCommand)
	a.Get("/robots", a.robots)
	a.Get("/robots/:robot", a.robot)
	a.Get("/robots/:robot/commands", a.robotCommands)
	a.Get(robotCommandRoute, a.executeRobotCommand)
	a.Post(robotCommandRoute, a.executeRobotCommand)
	a.Get("/robots/:robot/devices", a.robotDevices)
	a.Get("/robots/:robot/devices/:device", a.robotDevice)
	a.Get("/robots/:robot/devices/:device/commands", a.robotDeviceCommands)
	a.Get(deviceCommandRoute, a.executeDeviceCommand)
	a.Post(deviceCommandRoute, a.executeDeviceCommand)
	a.Get("/robots/:robot/connections", a.robotConnections)
	a.Get("/robots/:robot/connections/:connection", a.robotConnection)
	// robeaux
	a.Get("/index.html", a.robeaux)
	a.Get("/images/:a", a.robeaux)
	a.Get("/js/:a", a.robeaux)
	a.Get("/js/:a/", a.robeaux)
	a.Get("/js/:a/:b", a.robeaux)
	a.Get("/css/:a", a.robeaux)
	a.Get("/css/:a/", a.robeaux)
	a.Get("/css/:a/:b", a.robeaux)
	a.Get("/partials/:a", a.robeaux)

	a.start(a)
}

func (a *api) robeaux(res http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	buf, err := robeaux.Asset(path[1:])
	if err != nil {
		http.Error(res, err.Error(), http.StatusNotFound)
		return
	}
	t := strings.Split(path, ".")
	if t[len(t)-1] == "js" {
		res.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	} else if t[len(t)-1] == "css" {
		res.Header().Set("Content-Type", "text/css; charset=utf-8")
	}
	res.Write(buf)
}

func (a *api) mcp(res http.ResponseWriter, req *http.Request) {
	a.writeJSON(a.gobot.ToJSON(), res)
}

func (a *api) mcpCommands(res http.ResponseWriter, req *http.Request) {
	a.writeJSON(a.gobot.ToJSON().Commands, res)
}

func (a *api) robots(res http.ResponseWriter, req *http.Request) {
	jsonRobots := []*gobot.JSONRobot{}
	a.gobot.Robots().Each(func(r *gobot.Robot) {
		jsonRobots = append(jsonRobots, r.ToJSON())
	})
	a.writeJSON(jsonRobots, res)
}

func (a *api) robot(res http.ResponseWriter, req *http.Request) {
	a.writeJSON(a.gobot.Robot(req.URL.Query().Get(":robot")).ToJSON(), res)
}

func (a *api) robotCommands(res http.ResponseWriter, req *http.Request) {
	a.writeJSON(a.gobot.Robot(req.URL.Query().Get(":robot")).ToJSON().Commands, res)
}

func (a *api) robotDevices(res http.ResponseWriter, req *http.Request) {
	jsonDevices := []*gobot.JSONDevice{}
	a.gobot.Robot(req.URL.Query().Get(":robot")).Devices().Each(func(d gobot.Device) {
		jsonDevices = append(jsonDevices, d.ToJSON())
	})
	a.writeJSON(jsonDevices, res)
}

func (a *api) robotDevice(res http.ResponseWriter, req *http.Request) {
	a.writeJSON(
		a.gobot.Robot(req.URL.Query().Get(":robot")).
			Device(req.URL.Query().Get(":device")).ToJSON(), res,
	)
}

func (a *api) robotDeviceCommands(res http.ResponseWriter, req *http.Request) {
	a.writeJSON(
		a.gobot.Robot(req.URL.Query().Get(":robot")).
			Device(req.URL.Query().Get(":device")).ToJSON().Commands, res,
	)
}

func (a *api) robotConnections(res http.ResponseWriter, req *http.Request) {
	jsonConnections := []*gobot.JSONConnection{}
	a.gobot.Robot(req.URL.Query().Get(":robot")).Connections().Each(func(c gobot.Connection) {
		jsonConnections = append(jsonConnections, c.ToJSON())
	})
	a.writeJSON(jsonConnections, res)
}

func (a *api) robotConnection(res http.ResponseWriter, req *http.Request) {
	a.writeJSON(
		a.gobot.Robot(req.URL.Query().Get(":robot")).
			Connection(req.URL.Query().Get(":connection")).ToJSON(),
		res,
	)
}

func (a *api) executeMcpCommand(res http.ResponseWriter, req *http.Request) {
	a.executeCommand(a.gobot.Command(req.URL.Query().Get(":command")),
		res,
		req,
	)
}

func (a *api) executeDeviceCommand(res http.ResponseWriter, req *http.Request) {
	a.executeCommand(
		a.gobot.Robot(req.URL.Query().Get(":robot")).
			Device(req.URL.Query().Get(":device")).
			Command(req.URL.Query().Get(":command")),
		res,
		req,
	)
}

func (a *api) executeRobotCommand(res http.ResponseWriter, req *http.Request) {
	a.executeCommand(
		a.gobot.Robot(req.URL.Query().Get(":robot")).
			Command(req.URL.Query().Get(":command")),
		res,
		req,
	)
}

func (a *api) executeCommand(f func(map[string]interface{}) interface{},
	res http.ResponseWriter,
	req *http.Request,
) {

	body := make(map[string]interface{})
	json.NewDecoder(req.Body).Decode(&body)

	if f != nil {
		a.writeJSON(f(body), res)
	} else {
		a.writeJSON("Unknown Command", res)
	}

}

// basic auth inspired by
// https://github.com/codegangsta/martini-contrib/blob/master/auth/
func (a *api) basicAuth(res http.ResponseWriter, req *http.Request) {
	auth := req.Header.Get("Authorization")
	if !a.secureCompare(auth,
		"Basic "+base64.StdEncoding.EncodeToString([]byte(a.Username+":"+a.Password)),
	) {
		res.Header().Set("WWW-Authenticate",
			"Basic realm=\"Authorization Required\"",
		)
		http.Error(res, "Not Authorized", http.StatusUnauthorized)
	}
}

func (a *api) secureCompare(given string, actual string) bool {
	if subtle.ConstantTimeEq(int32(len(given)), int32(len(actual))) == 1 {
		return subtle.ConstantTimeCompare([]byte(given), []byte(actual)) == 1
	}
	// Securely compare actual to itself to keep constant time,
	// but always return false
	return subtle.ConstantTimeCompare([]byte(actual), []byte(actual)) == 1 && false
}

func (a *api) writeJSON(j interface{}, res http.ResponseWriter) {
	data, _ := json.Marshal(j)
	res.Header().Set("Content-Type", "application/json; charset=utf-8")
	res.Write(data)
}
