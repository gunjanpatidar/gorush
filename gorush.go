package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/appleboy/gorush/config"
	"github.com/appleboy/gorush/gorush"
)

func checkInput(token, message string) {
	if len(token) == 0 {
		gorush.LogError.Fatal("Missing token flag (-t)")
	}

	if len(message) == 0 {
		gorush.LogError.Fatal("Missing message flag (-m)")
	}
}

// Version control for gorush.
var Version = "No Version Provided"

var usageStr = `
Usage: gorush [options]

Server Options:
    -p, --port <port>                Use port for clients (default: 8088)
    -c, --config <file>              Configuration file
    -m, --message <message>          Notification message
    -t, --token <token>              Notification token
    --proxy <proxy>                  Proxy URL (only for GCM)
iOS Options:
    -i, --key <file>                 certificate key file path
    -P, --password <password>        certificate key password
    --topic <topic>                  iOS topic
    --ios                            enabled iOS (default: false)
    --production                     iOS production mode (default: false)
Android Options:
    -k, --apikey <api_key>           Android API Key
    --android                        enabled android (default: false)
Common Options:
    -h, --help                       Show this message
    -v, --version                    Show version
`

// usage will print out the flag options for the server.
func usage() {
	fmt.Printf("%s\n", usageStr)
	os.Exit(0)
}

func createPIDFile() error {
	if !gorush.PushConf.Core.PID.Enabled {
		return nil
	}
	_, err := os.Stat(gorush.PushConf.Core.PID.Path)
	if os.IsNotExist(err) || gorush.PushConf.Core.PID.Override {
		currentPid := os.Getpid()
		file, err := os.Create(gorush.PushConf.Core.PID.Path)
		if err != nil {
			return fmt.Errorf("Can't create PID file: %v", err)
		}
		defer file.Close()
		_, err = file.WriteString(strconv.FormatInt(int64(currentPid), 10))
		if err != nil {
			return fmt.Errorf("Can'write PID information on %s: %v", gorush.PushConf.Core.PID.Path, err)
		}
	} else {
		return fmt.Errorf("%s already exists", gorush.PushConf.Core.PID.Path)
	}
	return nil
}

func main() {

	var showVersion bool
	var configFile string
	var topic string
	var message string
	var token string
	var proxy string
	var app string
	var port string
	var iosKeyPath string
	var iosPassword string
	var iosEnabled bool
	var iosProduction bool
	var androidAPIKey string
	var androidEnabled bool

	flag.BoolVar(&showVersion, "version", false, "Print version information.")
	flag.BoolVar(&showVersion, "v", false, "Print version information.")
	flag.StringVar(&port, "p", "", "port number for gorush")
	flag.StringVar(&port, "port", "", "port number for gorush")

	flag.StringVar(&token, "t", "", "token string")
	flag.StringVar(&token, "token", "", "token string")

	flag.StringVar(&message, "m", "", "notification message")
	flag.StringVar(&message, "message", "", "notification message")

	flag.StringVar(&configFile, "c", "", "Configuration file.")
	flag.StringVar(&configFile, "config", "", "Configuration file.")

	flag.StringVar(&topic, "topic", "", "apns topic in iOS")
	flag.StringVar(&proxy, "proxy", "", "http proxy url")
	flag.StringVar(&app, "app", gorush.AppNameDefault, "app to use")

	flag.StringVar(&iosKeyPath, "i", "", "iOS certificate key file path")
	flag.StringVar(&iosKeyPath, "key", "", "iOS certificate key file path")
	flag.StringVar(&iosPassword, "P", "", "iOS certificate password for gorush")
	flag.StringVar(&iosPassword, "password", "", "iOS certificate password for gorush")
	flag.BoolVar(&iosEnabled, "ios", false, "send ios notification")
	flag.BoolVar(&iosProduction, "production", false, "production mode in iOS")

	flag.StringVar(&androidAPIKey, "k", "", "Android api key configuration for gorush")
	flag.StringVar(&androidAPIKey, "apikey", "", "Android api key configuration for gorush")
	flag.BoolVar(&androidEnabled, "android", false, "send android notification")

	flag.Usage = usage
	flag.Parse()

	gorush.SetVersion(Version)

	if len(os.Args) < 2 {
		usage()
	}

	// Show version and exit
	if showVersion {
		gorush.PrintGoRushVersion()
		os.Exit(0)
	}

	var err error

	// set default parameters.
	gorush.PushConf = config.BuildDefaultPushConf()

	// load user defined config.
	if configFile != "" {
		gorush.PushConf, err = config.LoadConfYaml(configFile)

		if err != nil {
			log.Printf("Load yaml config file error: '%v'", err)

			return
		}
	}

	// overwrite server port
	if port != "" {
		gorush.PushConf.Core.Port = port
	}

	// create a dynamic app from command line flags
	gorush.PushConf.Apps[gorush.AppNameDynamic] = config.SectionApp{}
	dynamicAppConfig := gorush.PushConf.Apps[gorush.AppNameDynamic]

	if iosKeyPath != "" {
		dynamicAppConfig.Ios.KeyPath = iosKeyPath
	}

	if iosPassword != "" {
		dynamicAppConfig.Ios.Password = iosPassword
	}

	if iosEnabled {
		dynamicAppConfig.Ios.Enabled = iosEnabled
	}

	if iosProduction {
		dynamicAppConfig.Ios.Production = iosProduction
	}

	if androidAPIKey != "" {
		dynamicAppConfig.Android.APIKey = androidAPIKey
	}

	if androidEnabled {
		dynamicAppConfig.Android.Enabled = androidEnabled
	}

	if err = gorush.InitLog(); err != nil {
		log.Println(err)

		return
	}

	// set http proxy for GCM
	if proxy != "" {
		err = gorush.SetProxy(proxy)

		if err != nil {
			gorush.LogError.Fatal("Set Proxy error: ", err)
		}
	} else if gorush.PushConf.Core.HTTPProxy != "" {
		err = gorush.SetProxy(gorush.PushConf.Core.HTTPProxy)

		if err != nil {
			gorush.LogError.Fatal("Set Proxy error: ", err)
		}
	}

	// send android notification
	if dynamicAppConfig.Android.Enabled {
		req := gorush.PushNotification{
			Tokens:   []string{token},
			Platform: gorush.PlatFormAndroid,
			Message:  message,
			AppID:    gorush.AppNameDynamic,
		}

		err := gorush.CheckMessage(req)

		if err != nil {
			gorush.LogError.Fatal(err)
		}

		gorush.InitAppStatus()
		gorush.PushToAndroid(req)

		return
	}

	// send ios notification
	if dynamicAppConfig.Ios.Enabled {
		req := gorush.PushNotification{
			Tokens:   []string{token},
			Platform: gorush.PlatFormIos,
			Message:  message,
			AppID:    gorush.AppNameDynamic,
		}

		if topic != "" {
			req.Topic = topic
		}

		err := gorush.CheckMessage(req)

		if err != nil {
			gorush.LogError.Fatal(err)
		}

		gorush.InitAppStatus()
		gorush.PushToIOS(req)

		return
	}

	if err = gorush.CheckPushConf(); err != nil {
		gorush.LogError.Fatal(err)
	}

	if err = createPIDFile(); err != nil {
		gorush.LogError.Fatal(err)
	}

	gorush.InitAppStatus()
	gorush.InitWorkers(int64(gorush.PushConf.Core.WorkerNum), int64(gorush.PushConf.Core.QueueNum))
	gorush.RunHTTPServer()
}
