# Webrtc SFU using pion

Methods 
```
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
```

### Method Join
При подключении нового участника просто создает Websocket соединение. Вызывает [функцию processJoin](https://github.com/HunterGooD/WebrtcSFU/blob/efc3caed55883de13f5f3fc465267b4e7574aea1/main.go#L301) которая отвечает на фронт с методом OnPublish и всеми pubPeers(транслирующие соединения). На фронте же создаются на каждый pubID свой Receiver.


### Method Subscribe 
Запрашивает подписку у всех кто раздает видео. Обмен с сервером sdp. На [фронте функция](https://github.com/HunterGooD/webrtcGolangServer/blob/main/web/src/classes/Webrtc/SFU.js#L113) И получение на сервере в [функции processSubscribe](https://github.com/HunterGooD/WebrtcSFU/blob/efc3caed55883de13f5f3fc465267b4e7574aea1/main.go#L195) Где пиру добавляется Track и генерируется ответ фронту.

### Method Publish
Раздает видео поток с фронта где на сервере вызывает [функцию processPublish](https://github.com/HunterGooD/WebrtcSFU/blob/efc3caed55883de13f5f3fc465267b4e7574aea1/main.go#L227) В которой записывается sdp для обмена с другими пользователями и добавляется в массив pub Peers и обменивается sdp со всеми кто находится в массиве sub Peers.

