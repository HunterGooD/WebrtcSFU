package media

import (
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/sirupsen/logrus"
)

var webrtcEngine *WebRTCEngine

func init() {
	webrtcEngine = NewWebRTCEngine()
}

type WebRTCPeer struct {
	UID      string
	PC       *webrtc.PeerConnection
	Track    *webrtc.TrackLocalStaticRTP
	stopChan chan int
	pliChan  chan int //pli picture loss indicator
}

func NewWebRTCPeer(uid string) *WebRTCPeer {
	return &WebRTCPeer{
		UID:      uid,
		stopChan: make(chan int),
		pliChan:  make(chan int),
	}
}

func (p *WebRTCPeer) Stop() {
	close(p.stopChan)
	close(p.pliChan)
}

func (p *WebRTCPeer) AnswerSender(offer webrtc.SessionDescription) (answer webrtc.SessionDescription, err error) {
	logrus.Infof("WebRTCPeer AnswerSender")
	return webrtcEngine.CreateReceiver(offer, &p.PC, &p.Track, p.stopChan, p.pliChan)
}

func (p *WebRTCPeer) AnswerReceiver(offer webrtc.SessionDescription, addTrack **webrtc.TrackLocalStaticRTP) (answer webrtc.SessionDescription, err error) {
	logrus.Infof("WebRTCPeer AnswerReceiver")
	return webrtcEngine.CreateSender(offer, &p.PC, addTrack)
}

func (p *WebRTCPeer) SendPLI() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("%v", r)
				return
			}
		}()
		ticker := time.NewTicker(time.Second)
		i := 0
		for {
			select {
			case <-ticker.C:
				p.pliChan <- 1
				if i > 3 {
					return
				}
				i++
			case <-p.stopChan:
				return
			}
		}
	}()
}
