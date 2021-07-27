package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"sync"
	"time"
	"webrtcGudov/internal/media"
	"webrtcGudov/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
	log "github.com/sirupsen/logrus"
)

const pingPeriod = 5 * time.Second

const (
	MethodJoin        = "join"
	MethodLeave       = "leave"
	MethodPublish     = "publish"
	MethodSubscribe   = "subscribe"
	MethodOnJoin      = "onJoin"
	MethodOnPublish   = "onPublish"
	MethodOnSubscribe = "onSubscribe"
	MethodOnUnpublish = "onUnpublish"
)

// Users
var (
	users = make(map[string]*User)

	pubPeers = make(map[string]*media.WebRTCPeer)
	pubLock  sync.RWMutex
	subPeers = make(map[string]*media.WebRTCPeer)
	subLock  sync.RWMutex
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func init() {
	log.SetOutput(os.Stdout)

	logLevel, err := log.ParseLevel(os.Getenv("LOG_LVL"))
	if err != nil {
		logLevel = log.DebugLevel
	}

	log.SetLevel(logLevel)
}

func main() {
	var port = ":3000"

	r := gin.Default()

	r.Any("/ws", handleWS)

	r.Run(port)
}

type User struct {
	Conn *websocket.Conn
	UID  string
	sync.Mutex
}

func (u *User) Close() {
	processLeave(u.UID)
}

func (t *User) WriteJSON(v interface{}) error {
	t.Lock()
	defer t.Unlock()

	return t.Conn.WriteJSON(v)
}

func (u *User) sendMessage(data model.Package) {
	u.WriteJSON(data)
}

func handleWS(c *gin.Context) {
	pingTicker := time.NewTicker(pingPeriod)

	uid := c.Query("uid")
	log.Infof("User %s connected", uid)
	if uid == "" {
		log.Errorf("Not query param uid")
		return
	}

	responseHeader := http.Header{}
	socket, err := upgrader.Upgrade(c.Writer, c.Request, responseHeader)
	if err != nil {
		panic(err)
	}

	var (
		in   = make(chan []byte)
		stop = make(chan struct{})
	)
	user := User{Conn: socket, UID: uid}
	go func() {
		for {
			_, message, err := user.Conn.ReadMessage()
			if err != nil {
				log.Errorf("Read socket Error %w", err)
				user.Close()
				close(stop)
				break
			}
			in <- message
		}
	}()

	// var pck *model.Package
	var pck model.Package
	for {
		select {
		case _ = <-pingTicker.C:
			if err := user.WriteJSON(
				&model.Package{
					Head: model.Head{
						Event: "heartPackage",
					}},
			); err != nil {
				pingTicker.Stop()
				return
			}
		case message := <-in:
			{
				log.Infof("Get message %s", message)
				if err := json.Unmarshal(message, &pck); err != nil {
					pingTicker.Stop()
					log.Errorf("Error on decode package json %s", message)
					return
				}
				var sdp webrtc.SessionDescription
				if err := json.Unmarshal([]byte(pck.Body.SDP), &sdp); err != nil {
					log.Warnf("Error parse sdp %s", message)
				}

				switch pck.Head.Event {
				case MethodJoin:
					processJoin(&user)
					break
				case MethodPublish:
					processPublish(&user, sdp)
					break
				case MethodSubscribe:
					processSubscribe(&user, sdp, pck.Head.PubID)
					break
				case MethodLeave:
					processJoin(&user)
					break
				default:
					{
						log.Errorf("Нет такого ивента %v", message)
					}
					break
				}
			}
		case <-stop:
			return
		}
	}
}

func processLeave(userId string) {
	mess := model.Package{
		Head: model.Head{
			Event: MethodOnUnpublish,
			PubID: userId,
		},
	}

	for id, user := range users {
		if id != userId {
			user.sendMessage(mess)
		}
	}
	deletePeer(userId, true)
	deletePeer(userId, false)
	delUser(userId)
}

func processSubscribe(u *User, sdp webrtc.SessionDescription, pubID string) {

	addPeer(u.UID, false)
	log.Infof("Generate Answer %s to %s", u.UID, pubID)
	answer, err := answer(u.UID, pubID, sdp, false)
	if err != nil {
		log.Errorf("Error create answer %v", err)
		return
	}
	sdpByte, err := json.Marshal(&answer)
	if err != nil {
		return
	}

	resp := model.Package{
		Head: model.Head{
			Event:  MethodOnSubscribe,
			UserID: u.UID,
			PubID:  pubID,
		},
		Body: model.Body{
			SDP: string(sdpByte),
		},
	}

	sendPLI(u.UID)

	u.sendMessage(resp)
	log.Infof("subs reciver id:%s", u.UID)
	return
}

func processPublish(u *User, sdp webrtc.SessionDescription) {

	addPeer(u.UID, true)

	answer, err := answer(u.UID, "", sdp, true)
	if err != nil {
		log.Errorf("Error create answer %v", err)
		return
	}
	sdpByte, err := json.Marshal(&answer)
	if err != nil {
		return
	}

	// resp := make(map[string]interface{})
	// resp["jsep"] = answer
	// resp["userID"] = u.UID
	resp := model.Package{
		Head: model.Head{
			Event:  MethodOnPublish,
			UserID: u.UID,
		},
		Body: model.Body{
			SDP: string(sdpByte),
		},
	}
	respByte, err := json.Marshal(&resp)
	if err != nil {
		return
	}
	respStr := string(respByte)
	if respStr != "" {

		u.sendMessage(resp)
		sendMessage(u, resp)
		return
	}
}

func answer(uid, peerID string, sdp webrtc.SessionDescription, sender bool) (webrtc.SessionDescription, error) {
	p := getPeer(uid, sender)

	var err error
	var answer webrtc.SessionDescription

	if sender {
		answer, err = p.AnswerSender(sdp)
	} else {
		pubLock.RLock()

		pub, ok := pubPeers[peerID]
		pubLock.RUnlock()
		if !ok {
			return webrtc.SessionDescription{}, errors.New("Нет pub peer id: " + peerID)
		}
		ticker := time.NewTicker(time.Second * 2)
		log.Infof("pub peer %#v", pub)
		for {
			select {
			case <-ticker.C:
				return p.AnswerReceiver(sdp, &pub.Track)
			default:
				if pub.Track == nil {
					time.Sleep(time.Millisecond * 200)
				} else {
					return p.AnswerReceiver(sdp, &pub.Track)
				}
			}
		}

	}
	return answer, err
}

func processJoin(u *User) {

	users[u.UID] = u

	mess := model.Package{
		Head: model.Head{
			Event: MethodOnPublish,
		},
	}

	pubLock.RLock()
	defer pubLock.RUnlock()
	for pubID := range pubPeers {
		if pubID != u.UID {
			mess.Head.PubID = pubID
			mess.Head.UserID = pubID
			u.sendMessage(mess)
		}
	}

	mess = model.Package{
		Head: model.Head{
			Event: MethodOnJoin,
		},
	}
	mess.Body.Data = "success"
	u.sendMessage(mess)

	log.Infof("%s user %s data %v", u.UID, MethodOnJoin, mess)
}

func delUser(id string) {
	delete(users, id)
}

func getPeer(id string, sender bool) *media.WebRTCPeer {
	if sender {
		pubLock.Lock()
		defer pubLock.Unlock()
		return pubPeers[id]
	} else {
		subLock.Lock()
		defer subLock.Unlock()
		return subPeers[id]
	}
}

func deletePeer(id string, sender bool) {
	if sender {
		pubLock.Lock()
		defer pubLock.Unlock()
		if _, ok := pubPeers[id]; ok {
			if pubPeers[id].PC != nil {
				pubPeers[id].PC.Close()
			}
			pubPeers[id].Stop()
		}
		delete(pubPeers, id)
	} else {
		subLock.Lock()
		defer subLock.Unlock()
		if _, ok := subPeers[id]; ok {
			if subPeers[id].PC != nil {
				subPeers[id].PC.Close()
			}
			subPeers[id].Stop()
		}
		delete(subPeers, id)
	}
}

func addPeer(id string, sender bool) {
	if sender {
		log.Infof("add pub peers id: %s", id)
		pubLock.Lock()
		defer pubLock.Unlock()
		if _, ok := pubPeers[id]; ok {
			pubPeers[id].Stop()
		}
		pubPeers[id] = media.NewWebRTCPeer(id)
	} else {
		log.Infof("add sub peers id: %s", id)
		subLock.Lock()
		defer subLock.Unlock()
		if _, ok := subPeers[id]; ok {
			subPeers[id].Stop()
		}
		subPeers[id] = media.NewWebRTCPeer(id)
	}
}

func sendPLI(skipID string) {
	log.Infof("Send PLI")
	pubLock.RLock()
	defer pubLock.RUnlock()
	for k, v := range pubPeers {
		if k != skipID {
			v.SendPLI()
		}
	}
}

func sendMessage(from *User, message model.Package) {
	// var message = model.Package{
	// 	Head: model.Head{
	// 		Event:  msgType,
	// 		UserID: from.UID,
	// 	},
	// 	Body: model.Body{
	// 		Data: data,
	// 	},
	// }

	for id, user := range users {
		if id != from.UID {
			user.WriteJSON(message)
		}
	}
}
