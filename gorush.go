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

// AppNameDefault is the name for the default app in the config
var AppNameDefault = "default"

// AppNameDynamic is the key for config provided through the command line flags in gorush.PushConf
var AppNameDynamic = "__dynamic_app"

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
	opts := config.ConfYaml{}

	var showVersion bool
	var configFile string
	var topic string
	var message string
	var token string
	var proxy string
	var app string

	flag.BoolVar(&showVersion, "version", false, "Print version information.")
	flag.BoolVar(&showVersion, "v", false, "Print version information.")
	flag.StringVar(&opts.Core.Port, "p", "", "port number for gorush")
	flag.StringVar(&opts.Core.Port, "port", "", "port number for gorush")

	flag.StringVar(&token, "t", "", "token string")
	flag.StringVar(&token, "token", "", "token string")

	flag.StringVar(&message, "m", "", "notification message")
	flag.StringVar(&message, "message", "", "notification message")

	flag.StringVar(&configFile, "c", "", "Configuration file.")
	flag.StringVar(&configFile, "config", "", "Configuration file.")

	flag.StringVar(&topic, "topic", "", "apns topic in iOS")
	flag.StringVar(&proxy, "proxy", "", "http proxy url")
	flag.StringVar(&app, "app", AppNameDefault, "app to use")

	flag.StringVar(&opts.Ios.KeyPath, "i", "", "iOS certificate key file path")
	flag.StringVar(&opts.Ios.KeyPath, "key", "", "iOS certificate key file path")
	flag.StringVar(&opts.Ios.Password, "P", "", "iOS certificate password for gorush")
	flag.StringVar(&opts.Ios.Password, "password", "", "iOS certificate password for gorush")
	flag.BoolVar(&opts.Ios.Enabled, "ios", false, "send ios notification")
	flag.BoolVar(&opts.Ios.Production, "production", false, "production mode in iOS")

	flag.StringVar(&opts.Android.APIKey, "k", "", "Android api key configuration for gorush")
	flag.StringVar(&opts.Android.APIKey, "apikey", "", "Android api key configuration for gorush")
	flag.BoolVar(&opts.Android.Enabled, "android", false, "send android notification")

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

	// load user define config.
	if configFile != "" {
		gorush.PushConf, err = config.LoadConfYaml(configFile)

		if err != nil {
			log.Printf("Load yaml config file error: '%v'", err)

			return
		}
	}

	// overwrite server port
	if opts.Core.Port != "" {
		gorush.PushConf.Core.Port = opts.Core.Port
	}

	// create a dynamic app from command line flags
	gorush.PushConf.Apps[AppNameDynamic] = config.SectionApp{}
	dynamicAppConfig := gorush.PushConf.Apps[AppNameDynamic]

	if opts.Ios.KeyPath != "" {
		dynamicAppConfig.Ios.KeyPath = opts.Ios.KeyPath
	}

	if opts.Ios.Password != "" {
		dynamicAppConfig.Ios.Password = opts.Ios.Password
	}

	if opts.Ios.Enabled {
		dynamicAppConfig.Ios.Enabled = opts.Ios.Enabled
	}

	if opts.Ios.Production {
		gorush.PushConf.Ios.Production = opts.Ios.Production
	}

	if opts.Android.APIKey != "" {
		dynamicAppConfig.Android.APIKey = opts.Android.APIKey
	}

	if opts.Android.Enabled {
		dynamicAppConfig.Android.Enabled = opts.Android.Enabled
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
		}

		if topic != "" {
			req.Topic = topic
		}

		err := gorush.CheckMessage(req)

		if err != nil {
			gorush.LogError.Fatal(err)
		}

		gorush.InitAppStatus()
		gorush.InitAPNSClient()
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
	gorush.InitAPNSClient()
	gorush.InitWorkers(int64(gorush.PushConf.Core.WorkerNum), int64(gorush.PushConf.Core.QueueNum))
	gorush.RunHTTPServer()
}
