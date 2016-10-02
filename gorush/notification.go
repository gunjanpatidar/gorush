package gorush

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/go-gcm"
	apns "github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	"github.com/sideshow/apns2/payload"
)

// D provide string array
type D map[string]interface{}

const (
	// ApnsPriorityLow will tell APNs to send the push message at a time that takes
	// into account power considerations for the device. Notifications with this
	// priority might be grouped and delivered in bursts. They are throttled, and
	// in some cases are not delivered.
	ApnsPriorityLow = 5

	// ApnsPriorityHigh will tell APNs to send the push message immediately.
	// Notifications with this priority must trigger an alert, sound, or badge on
	// the target device. It is an error to use this priority for a push
	// notification that contains only the content-available key.
	ApnsPriorityHigh = 10
)

// Alert is APNs payload
type Alert struct {
	Action       string   `json:"action,omitempty"`
	ActionLocKey string   `json:"action-loc-key,omitempty"`
	Body         string   `json:"body,omitempty"`
	LaunchImage  string   `json:"launch-image,omitempty"`
	LocArgs      []string `json:"loc-args,omitempty"`
	LocKey       string   `json:"loc-key,omitempty"`
	Title        string   `json:"title,omitempty"`
	TitleLocArgs []string `json:"title-loc-args,omitempty"`
	TitleLocKey  string   `json:"title-loc-key,omitempty"`
}

// RequestPush support multiple notification request.
type RequestPush struct {
	Notifications []PushNotification `json:"notifications" binding:"required"`
}

// PushNotification is single notification request
type PushNotification struct {
	// Common
	Tokens           []string `json:"tokens" binding:"required"`
	Platform         int      `json:"platform" binding:"required"`
	Message          string   `json:"message" binding:"required"`
	Title            string   `json:"title,omitempty"`
	Priority         string   `json:"priority,omitempty"`
	ContentAvailable bool     `json:"content_available,omitempty"`
	Sound            string   `json:"sound,omitempty"`
	Data             D        `json:"data,omitempty"`
	AppID            string   `json:"data,omitempty"`

	// Android
	APIKey                string           `json:"api_key,omitempty"`
	To                    string           `json:"to,omitempty"`
	CollapseKey           string           `json:"collapse_key,omitempty"`
	DelayWhileIdle        bool             `json:"delay_while_idle,omitempty"`
	TimeToLive            *uint            `json:"time_to_live,omitempty"`
	RestrictedPackageName string           `json:"restricted_package_name,omitempty"`
	DryRun                bool             `json:"dry_run,omitempty"`
	Notification          gcm.Notification `json:"notification,omitempty"`

	// iOS
	Expiration int64    `json:"expiration,omitempty"`
	ApnsID     string   `json:"apns_id,omitempty"`
	Topic      string   `json:"topic,omitempty"`
	Badge      int      `json:"badge,omitempty"`
	Category   string   `json:"category,omitempty"`
	URLArgs    []string `json:"url-args,omitempty"`
	Alert      Alert    `json:"alert,omitempty"`
}

// ApnsClients is collection of apns client connections
type ApnsClients struct {
	lock    sync.RWMutex
	clients map[string]*apns.Client
}

var apnsClients = &ApnsClients{}

// CheckMessage for check request message
func CheckMessage(req PushNotification) error {
	var msg string
	if req.Message == "" {
		msg = "the message must not be empty"
		LogAccess.Debug(msg)
		return errors.New(msg)
	}

	if len(req.Tokens) == 0 {
		msg = "the message must specify at least one registration ID"
		LogAccess.Debug(msg)
		return errors.New(msg)
	}

	// TODO: Looks Wrong
	if len(req.Tokens) == PlatFormIos && len(req.Tokens[0]) == 0 {
		msg = "the token must not be empty"
		LogAccess.Debug(msg)
		return errors.New(msg)
	}

	if req.Platform == PlatFormAndroid && len(req.Tokens) > 1000 {
		msg = "the message may specify at most 1000 registration IDs"
		LogAccess.Debug(msg)
		return errors.New(msg)
	}

	// ref: https://developers.google.com/cloud-messaging/http-server-ref
	if req.Platform == PlatFormAndroid && req.TimeToLive != nil && (*req.TimeToLive < uint(0) || uint(2419200) < *req.TimeToLive) {
		msg = "the message's TimeToLive field must be an integer " +
			"between 0 and 2419200 (4 weeks)"
		LogAccess.Debug(msg)
		return errors.New(msg)
	}

	return nil
}

// SetProxy only working for GCM server.
func SetProxy(proxy string) error {

	proxyURL, err := url.ParseRequestURI(proxy)

	if err != nil {
		return err
	}

	http.DefaultTransport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	LogAccess.Debug("Set http proxy as " + proxy)

	return nil
}

// CheckPushConf provide check your yml config.
// TODO: needs reimplementation
func CheckPushConf() error {
	/*
		if !PushConf.Ios.Enabled && !PushConf.Android.Enabled {
			return errors.New("Please enable iOS or Android config in yml config")
		}

		if PushConf.Ios.Enabled {
			if PushConf.Ios.KeyPath == "" {
				return errors.New("Missing iOS certificate path")
			}
		}

		if PushConf.Android.Enabled {
			if PushConf.Android.APIKey == "" {
				return errors.New("Missing Android API Key")
			}
		}
	*/
	return nil
}

// initAPNSClient initializes an APNs Client for the given AppID.
func initAPNSClient(AppID string) (*apns.Client, error) {
	var err error
	var apnsClient *apns.Client

	if PushConf.Apps[AppID].Ios.Enabled {
		ext := filepath.Ext(PushConf.Apps[AppID].Ios.KeyPath)

		switch ext {
		case ".p12":
			CertificatePemIos, err = certificate.FromP12File(PushConf.Apps[AppID].Ios.KeyPath, PushConf.Apps[AppID].Ios.Password)
		case ".pem":
			CertificatePemIos, err = certificate.FromPemFile(PushConf.Apps[AppID].Ios.KeyPath, PushConf.Apps[AppID].Ios.Password)
		default:
			err = errors.New("Wrong Certificate key extension.")
		}

		if err != nil {
			LogError.Error("Cert Error:", err.Error())

			return nil, err
		}

		if PushConf.Apps[AppID].Ios.Production {
			apnsClient = apns.NewClient(CertificatePemIos).Production()
		} else {
			apnsClient = apns.NewClient(CertificatePemIos).Development()
		}
	}

	return apnsClient, err
}

// GetAPNSClient returns an existing APNs client connection if available else
// creates a new connection and returns
//
// For faster concurrency with locks, double checks have been used
// (https://www.misfra.me/optimizing-concurrent-map-access-in-go/)
func GetAPNSClient(AppID string) (*apns.Client, error) {
	var client *apns.Client
	var present bool
	var err error

	apnsClients.lock.RLock()
	if client, present = apnsClients.clients[AppID]; !present {
		// The connection wasn't found, so we'll create it.
		apnsClients.lock.RUnlock()
		apnsClients.lock.Lock()
		if client, present = apnsClients.clients[AppID]; !present {
			client, err = initAPNSClient(AppID)

			apnsClients.clients[AppID] = client
		}
		apnsClients.lock.Unlock()
	} else {
		apnsClients.lock.RUnlock()
	}

	return client, err
}

// InitWorkers for initialize all workers.
func InitWorkers(workerNum int64, queueNum int64) {
	LogAccess.Debug("worker number is ", workerNum, ", queue number is ", queueNum)
	QueueNotification = make(chan PushNotification, queueNum)
	for i := int64(0); i < workerNum; i++ {
		go startWorker()
	}
}

func startWorker() {
	for {
		notification := <-QueueNotification
		switch notification.Platform {
		case PlatFormIos:
			PushToIOS(notification)
		case PlatFormAndroid:
			PushToAndroid(notification)
		}
	}
}

// queueNotification add notification to queue list.
func queueNotification(req RequestPush) int {
	var count int
	for _, notification := range req.Notifications {

		// send notification to `default` app, if app not specified
		if notification.AppID == "" {
			notification.AppID = AppNameDefault
		}

		// skip notification if unkown app specified
		if _, exists := PushConf.Apps[notification.AppID]; !exists {
			LogError.Error("Unknown app: " + notification.AppID)
			continue
		}

		switch notification.Platform {
		case PlatFormIos:
			if !PushConf.Apps[notification.AppID].Ios.Enabled {
				continue
			}
		case PlatFormAndroid:
			if !PushConf.Apps[notification.AppID].Android.Enabled {
				continue
			}
		}
		QueueNotification <- notification

		count += len(notification.Tokens)
	}

	StatStorage.AddTotalCount(int64(count))

	return count
}

func iosAlertDictionary(payload *payload.Payload, req PushNotification) *payload.Payload {
	// Alert dictionary

	if len(req.Title) > 0 {
		payload.AlertTitle(req.Title)
	}

	if len(req.Alert.TitleLocKey) > 0 {
		payload.AlertTitleLocKey(req.Alert.TitleLocKey)
	}

	if len(req.Alert.LocArgs) > 0 {
		payload.AlertLocArgs(req.Alert.LocArgs)
	}

	if len(req.Alert.TitleLocArgs) > 0 {
		payload.AlertTitleLocArgs(req.Alert.TitleLocArgs)
	}

	if len(req.Alert.Body) > 0 {
		payload.AlertBody(req.Alert.Body)
	}

	if len(req.Alert.LaunchImage) > 0 {
		payload.AlertLaunchImage(req.Alert.LaunchImage)
	}

	if len(req.Alert.LocKey) > 0 {
		payload.AlertLocKey(req.Alert.LocKey)
	}

	if len(req.Alert.Action) > 0 {
		payload.AlertAction(req.Alert.Action)
	}

	if len(req.Alert.ActionLocKey) > 0 {
		payload.AlertActionLocKey(req.Alert.ActionLocKey)
	}

	// General

	if len(req.Category) > 0 {
		payload.Category(req.Category)
	}

	return payload
}

// GetIOSNotification use for define iOS notificaiton.
// The iOS Notification Payload
// ref: https://developer.apple.com/library/ios/documentation/NetworkingInternet/Conceptual/RemoteNotificationsPG/Chapters/TheNotificationPayload.html
func GetIOSNotification(req PushNotification) *apns.Notification {
	notification := &apns.Notification{
		ApnsID: req.ApnsID,
		Topic:  req.Topic,
	}

	if req.Expiration > 0 {
		notification.Expiration = time.Unix(req.Expiration, 0)
	}

	if len(req.Priority) > 0 && req.Priority == "normal" {
		notification.Priority = apns.PriorityLow
	}

	payload := payload.NewPayload().Alert(req.Message)

	if req.Badge > 0 {
		payload.Badge(req.Badge)
	}

	if len(req.Sound) > 0 {
		payload.Sound(req.Sound)
	}

	if req.ContentAvailable {
		payload.ContentAvailable()
	}

	if len(req.URLArgs) > 0 {
		payload.URLArgs(req.URLArgs)
	}

	for k, v := range req.Data {
		payload.Custom(k, v)
	}

	payload = iosAlertDictionary(payload, req)

	notification.Payload = payload

	return notification
}

// PushToIOS provide send notification to APNs server.
func PushToIOS(req PushNotification) bool {
	LogAccess.Debug("Start push notification for iOS")

	var isError bool

	notification := GetIOSNotification(req)

	// get apns client
	apnsClient, err := GetAPNSClient(req.AppID)
	if err != nil {
		LogPush(FailedPush, "", req, err)
		isError = true
		return isError
	}

	for _, token := range req.Tokens {
		notification.DeviceToken = token

		// send ios notification
		res, err := apnsClient.Push(notification)
		if err != nil {
			// apns server error
			LogPush(FailedPush, token, req, err)
			isError = true
			StatStorage.AddIosError(1)
			continue
		}

		if res.StatusCode != 200 {
			// error message:
			// ref: https://github.com/sideshow/apns2/blob/master/response.go#L14-L65
			LogPush(FailedPush, token, req, errors.New(res.Reason))
			StatStorage.AddIosError(1)
			continue
		}

		if res.Sent() {
			LogPush(SucceededPush, token, req, nil)
			StatStorage.AddIosSuccess(1)
		}
	}

	return isError
}

// GetAndroidNotification use for define Android notificaiton.
// HTTP Connection Server Reference for Android
// https://developers.google.com/cloud-messaging/http-server-ref
func GetAndroidNotification(req PushNotification) gcm.HttpMessage {
	notification := gcm.HttpMessage{
		To:                    req.To,
		CollapseKey:           req.CollapseKey,
		ContentAvailable:      req.ContentAvailable,
		DelayWhileIdle:        req.DelayWhileIdle,
		TimeToLive:            req.TimeToLive,
		RestrictedPackageName: req.RestrictedPackageName,
		DryRun:                req.DryRun,
	}

	notification.RegistrationIds = req.Tokens

	if len(req.Priority) > 0 && req.Priority == "high" {
		notification.Priority = "high"
	}

	// Add another field
	if len(req.Data) > 0 {
		notification.Data = make(map[string]interface{})
		for k, v := range req.Data {
			notification.Data[k] = v
		}
	}

	notification.Notification = &req.Notification

	// Set request message if body is empty
	if len(notification.Notification.Body) == 0 {
		notification.Notification.Body = req.Message
	}

	if len(req.Title) > 0 {
		notification.Notification.Title = req.Title
	}

	if len(req.Sound) > 0 {
		notification.Notification.Sound = req.Sound
	}

	return notification
}

// PushToAndroid provide send notification to Android server.
func PushToAndroid(req PushNotification) bool {
	LogAccess.Debug("Start push notification for Android")

	// check message
	err := CheckMessage(req)
	if err != nil {
		LogError.Error("request error: " + err.Error())
		return false
	}

	notification := GetAndroidNotification(req)

	res, err := gcm.SendHttp(req.APIKey, notification)
	if err != nil {
		// GCM server error
		LogError.Error("GCM server error: " + err.Error())

		return false
	}

	LogAccess.Debug(fmt.Sprintf("Android Success count: %d, Failure count: %d", res.Success, res.Failure))
	StatStorage.AddAndroidSuccess(int64(res.Success))
	StatStorage.AddAndroidError(int64(res.Failure))

	for k, result := range res.Results {
		if result.Error != "" {
			LogPush(FailedPush, req.Tokens[k], req, errors.New(result.Error))
			continue
		}

		LogPush(SucceededPush, req.Tokens[k], req, nil)
	}

	return true
}
