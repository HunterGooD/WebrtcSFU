package media

import (
	"log"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
	"github.com/sirupsen/logrus"
)

var defaultConfiguration = webrtc.Configuration{
	SDPSemantics: webrtc.SDPSemanticsUnifiedPlanWithFallback,
	ICEServers: []webrtc.ICEServer{
		{
			URLs: []string{"stun:stun.stunprotocol.org:3478"},
		},
	},
}

type WebRTCEngine struct {
	conf        webrtc.Configuration
	mediaEngine *webrtc.MediaEngine
	api         *webrtc.API
}

func NewWebRTCEngine() *WebRTCEngine {
	w := &WebRTCEngine{
		conf: defaultConfiguration,
	}

	m := &webrtc.MediaEngine{}

	// if err := m.RegisterCodec(webrtc.RTPCodecParameters{
	// 	RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil},
	// 	PayloadType:        96,
	// }, webrtc.RTPCodecTypeVideo); err != nil {
	// 	panic(err)
	// }
	// if err := m.RegisterCodec(webrtc.RTPCodecParameters{
	// 	RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2, SDPFmtpLine: "minptime=10; useinbandfec=1", RTCPFeedback: nil},
	// 	PayloadType:        111,
	// }, webrtc.RTPCodecTypeAudio); err != nil {
	// 	panic(err)
	// }

	if err := m.RegisterDefaultCodecs(); err != nil {
		panic(err)
	}

	w.api = webrtc.NewAPI(webrtc.WithMediaEngine(m))

	return w
}

func (e WebRTCEngine) CreateSender(offer webrtc.SessionDescription, pc **webrtc.PeerConnection, track **webrtc.TrackLocalStaticRTP) (webrtc.SessionDescription, error) {
	var err error
	*pc, err = e.api.NewPeerConnection(e.conf)
	if err != nil {
		return webrtc.SessionDescription{}, err
	}

	for *track == nil {
		time.Sleep(time.Millisecond * 100)
	}
	logrus.Infof("[TRACK] %p, %#v", *track, **track)
	logrus.Infof("Set track on peer connection ")
	(*pc).AddTrack(*track)
	err = (*pc).SetRemoteDescription(offer)
	if err != nil {
		return webrtc.SessionDescription{}, err
	}

	answer, err := (*pc).CreateAnswer(nil)
	err = (*pc).SetLocalDescription(answer)
	if err != nil {
		return webrtc.SessionDescription{}, err
	}
	return answer, nil
}

func (e WebRTCEngine) CreateReceiver(offer webrtc.SessionDescription, pc **webrtc.PeerConnection, track **webrtc.TrackLocalStaticRTP, pliChan, sChan chan int) (webrtc.SessionDescription, error) {
	var err error
	*pc, err = e.api.NewPeerConnection(e.conf)
	if err != nil {
		return webrtc.SessionDescription{}, err
	}

	for _, typ := range []webrtc.RTPCodecType{webrtc.RTPCodecTypeVideo, webrtc.RTPCodecTypeAudio} {
		if _, err := (*pc).AddTransceiverFromKind(typ, webrtc.RTPTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionRecvonly,
		}); err != nil {
			log.Print(err)
			return webrtc.SessionDescription{}, err
		}
	}

	(*pc).OnTrack(func(t *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		*track, err = webrtc.NewTrackLocalStaticRTP(t.Codec().RTPCodecCapability, t.ID(), t.StreamID())

		// dispath key frames
		go func() {
			for {
				select {
				case <-pliChan:
					(*pc).WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{
						MediaSSRC: uint32(t.SSRC()),
					}})
				case <-sChan:
					return
				}
			}
		}()

		buf := make([]byte, 1500)
		for {
			i, _, err := t.Read(buf)
			if err != nil {
				return
			}

			if _, err = (*track).Write(buf[:i]); err != nil {
				return
			}
		}

	})

	err = (*pc).SetRemoteDescription(offer)
	if err != nil {
		return webrtc.SessionDescription{}, err
	}

	answer, err := (*pc).CreateAnswer(nil)
	err = (*pc).SetLocalDescription(answer)
	logrus.Infof("Create receiver ok")

	return answer, err
}
