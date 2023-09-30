package ws

import (
	"encoding/json"
	"fmt"
	"goseg/auth"
	"goseg/broadcast"
	"goseg/config"
	"goseg/handler"
	"goseg/logger"
	"goseg/setup"
	"goseg/startram"
	"goseg/structs"
	"net/http"
	"strings"

	// "time"

	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins
		},
	}
)

// switch on ws event cases
// func WsHandler(w http.ResponseWriter, r *http.Request) {
// 	conn, err := upgrader.Upgrade(w, r, nil)
// 	if err != nil {
// 		logger.Logger.Error(fmt.Sprintf("Couldn't upgrade websocket connection: %v", err))
// 		return
// 	}
// 	// tokenId := config.RandString(32)
// 	// MuCon := auth.ClientManager.NewConnection(conn, tokenId)
// 	for {
// 		var handleLoop string
// 		if system.IsC2CMode() {
// 			handleLoop = c2cHandler(w, r, conn)
// 		} else {
// 			handleLoop = mainWSHandler(w, r, conn)
// 		}
// 		if handleLoop == "break" {
// 			break
// 		}
// 		if handleLoop == "continue" {
// 			continue
// 		}
// 	}
// }

func c2cHandler(w http.ResponseWriter, r *http.Request, conn *websocket.Conn) string {
	_, msg, err := conn.ReadMessage()
	if err != nil {
		if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived) || strings.Contains(err.Error(), "broken pipe") {
			logger.Logger.Info("WS closed")
			conn.Close()
			auth.WsNilSession(conn)
		}
		logger.Logger.Warn(fmt.Sprintf("Error reading websocket message: %v", err))
		return "break"
	}
	var payload structs.C2CPayload
	if err := json.Unmarshal(msg, &payload); err != nil {
		logger.Logger.Error(fmt.Sprintf("Error unmarshalling C2C payload: %v", err))
		return "continue"
	}
	resp, err := handler.C2CHandler(payload)
	if err != nil {
		logger.Logger.Warn(fmt.Sprintf("Unable to generate c2c payload: %v", err))
	}
	MuCon := auth.ClientManager.NewConnection(conn, "somebody-once-told-me")
	MuCon.Write(resp)
	return ""
}

func WsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Logger.Error(fmt.Sprintf("Couldn't upgrade websocket connection: %v", err))
		return
	}
	// tokenId := config.RandString(32)
	// MuCon := auth.ClientManager.NewConnection(conn, tokenId)
	_, msg, err := conn.ReadMessage()
	if err != nil {
		if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived) || strings.Contains(err.Error(), "broken pipe") {
			logger.Logger.Info("WS closed")
			conn.Close()
			// cancel all log streams for this ws
			// logEvent := structs.LogsEvent{
			// 	Action:      false,
			// 	ContainerID: "all",
			// 	MuCon:       MuCon,
			// }
			// config.LogsEventBus <- logEvent
			// mute the session
			auth.WsNilSession(conn)
		}
		logger.Logger.Warn(fmt.Sprintf("Error reading websocket message: %v", err))
		return "break"
	}
	var payload structs.WsPayload
	if err := json.Unmarshal(msg, &payload); err != nil {
		logger.Logger.Error(fmt.Sprintf("Error unmarshalling payload: %v", err))
		return "continue"
	}
	MuCon := auth.ClientManager.NewConnection(conn, payload.Token.ID)
	var msgType structs.WsType
	err = json.Unmarshal(msg, &msgType)
	if err != nil {
		logger.Logger.Error(fmt.Sprintf("Error marshalling token (else): %v", err))
		return "continue"
	}
	token := map[string]string{
		"id":    payload.Token.ID,
		"token": payload.Token.Token,
	}
	tokenContent, authed := auth.CheckToken(token, conn, r)
	token = map[string]string{
		"id":    payload.Token.ID,
		"token": tokenContent,
	}
	ack := "ack"
	conf := config.Conf()
	if authed || conf.Setup != "complete" {
		// send setup broadcast if we're not done setting up
		if conf.Setup != "complete" {
			resp := structs.SetupBroadcast{
				Type:      "structure",
				AuthLevel: "setup",
				Stage:     conf.Setup,
				Page:      setup.Stages[conf.Setup],
				Regions:   startram.Regions,
			}
			respJSON, err := json.Marshal(resp)
			if err != nil {
				logger.Logger.Error(fmt.Sprintf("Couldn't marshal startram regions: %v", err))
			}
			MuCon.Write(respJSON)
		}
		switch msgType.Payload.Type {
		case "new_ship":
			if err = handler.NewShipHandler(msg); err != nil {
				logger.Logger.Error(fmt.Sprintf("%v", err))
				ack = "nack"
			}
		case "pier_upload":
			if err = handler.UploadHandler(msg); err != nil {
				logger.Logger.Error(fmt.Sprintf("%v", err))
				ack = "nack"
			}
		case "password":
			if err = handler.PwHandler(msg); err != nil {
				logger.Logger.Error(fmt.Sprintf("%v", err))
				ack = "nack"
			}
		case "system":
			if err = handler.SystemHandler(msg); err != nil {
				logger.Logger.Error(fmt.Sprintf("%v", err))
				ack = "nack"
			}
		case "startram":
			if err = handler.StartramHandler(msg); err != nil {
				logger.Logger.Error(fmt.Sprintf("%v", err))
				ack = "nack"
			}
		case "urbit":
			if err = handler.UrbitHandler(msg); err != nil {
				logger.Logger.Error(fmt.Sprintf("%v", err))
				ack = "nack"
			}
		case "support":
			if err := handler.SupportHandler(msg, payload); err != nil {
				logger.Logger.Error(fmt.Sprintf("Error creating bug report: %v", err))
				ack = "nack"
			}
		case "broadcast":
			if err := broadcast.BroadcastToClients(); err != nil {
				logger.Logger.Error(fmt.Sprintf("Unable to broadcast to clients: %v", err))
				ack = "nack"
			}
		case "login":
			// already authed so lets get you on the map
			if err := auth.AddToAuthMap(conn, token, true); err != nil {
				logger.Logger.Error(fmt.Sprintf("Unable to reauth: %v", err))
				ack = "nack"
			}
			if err := broadcast.BroadcastToClients(); err != nil {
				logger.Logger.Error(fmt.Sprintf("Unable to broadcast to clients: %v", err))
				ack = "nack"
			}
		case "logout":
			if err := handler.LogoutHandler(msg); err != nil {
				logger.Logger.Error(fmt.Sprintf("Error logging out client: %v", err))
				ack = "nack"
			}
			resp, err := handler.UnauthHandler()
			if err != nil {
				logger.Logger.Warn(fmt.Sprintf("Unable to generate deauth payload: %v", err))
			}
			MuCon.Write(resp)
		case "verify":
			authed := true
			if conf.Setup != "complete" {
				authed = false
			}
			if err := auth.AddToAuthMap(conn, token, authed); err != nil {
				logger.Logger.Error(fmt.Sprintf("Unable to reauth: %v", err))
				ack = "nack"
			}
			if !authed {
				resp, err := handler.UnauthHandler()
				if err != nil {
					logger.Logger.Warn(fmt.Sprintf("Unable to generate deauth payload: %v", err))
				}
				MuCon.Write(resp)
			}
			if err := broadcast.BroadcastToClients(); err != nil {
				logger.Logger.Error(fmt.Sprintf("Unable to broadcast to clients: %v", err))
			}
		case "logs":
			var logPayload structs.WsLogsPayload
			if err := json.Unmarshal(msg, &logPayload); err != nil {
				logger.Logger.Error(fmt.Sprintf("Error unmarshalling payload: %v", err))
				return "continue"
			}
			logEvent := structs.LogsEvent{
				Action:      logPayload.Payload.Action,
				ContainerID: logPayload.Payload.ContainerID,
				MuCon:       MuCon,
			}
			config.LogsEventBus <- logEvent
		case "setup":
			if err = setup.Setup(msg, MuCon, token); err != nil {
				logger.Logger.Error(fmt.Sprintf("%v", err))
				ack = "nack"
			}
			conf = config.Conf()
		default:
			errmsg := fmt.Sprintf("Unknown auth request type: %s", msgType.Payload.Type)
			logger.Logger.Warn(errmsg)
			if err := broadcast.BroadcastToClients(); err != nil {
				logger.Logger.Error(fmt.Sprintf("Unable to broadcast to clients: %v", err))
			}
			ack = "nack"
		}
		// ack/nack for auth
		result := map[string]interface{}{
			"type":     "activity",
			"id":       payload.ID,
			"error":    "null",
			"response": ack,
			"token":    token,
		}
		respJson, err := json.Marshal(result)
		if err != nil {
			errmsg := fmt.Sprintf("Error marshalling token (init): %v", err)
			logger.Logger.Error(errmsg)
		}
		MuCon.Write(respJson)
		// unauthenticated action handlers
	} else {
		switch msgType.Payload.Type {
		case "login":
			if err = handler.LoginHandler(MuCon, msg); err != nil {
				logger.Logger.Error(fmt.Sprintf("%v", err))
				ack = "nack"
			}
			broadcast.BroadcastToClients()
		case "verify":
			logger.Logger.Info("New client")
			// auth.CreateToken also adds to unauth map
			newToken, err := auth.CreateToken(conn, r, false)
			if err != nil {
				logger.Logger.Error(fmt.Sprintf("Unable to create token: %v", err))
				ack = "nack"
			}
			token = newToken
		default:
			resp, err := handler.UnauthHandler()
			if err != nil {
				logger.Logger.Warn(fmt.Sprintf("Unable to generate deauth payload: %v", err))
			}
			MuCon.Write(resp)
			ack = "nack"
		}
		// ack/nack for unauth broadcast
		result := map[string]interface{}{
			"type":     "activity",
			"id":       payload.ID,
			"error":    "null",
			"response": ack,
			"token":    token,
		}
		respJson, err := json.Marshal(result)
		if err != nil {
			logger.Logger.Error(fmt.Sprintf("Error marshalling token (init): %v", err))
		}
		MuCon.Write(respJson)
	}
	return ""
}
