package main

import (
	"fmt"
	"math/rand"

	"github.com/go-macaron/binding"
	"gopkg.in/macaron.v1"
)

func main() {
	m := macaron.Classic()
	m.Use(macaron.Renderer())

	m.Group("/room", func() {
		m.Get("/:roomid", roomGET)
		m.Post("/:roomid", binding.Bind(messageForm{}), roomPOST)
		m.Delete("/:roomid", roomDELETE)
	})

	m.Run("localhost", 8082)
}

func roomGET(ctx *macaron.Context) {
	ctx.Data["roomid"] = ctx.Params("roomid")
	ctx.Data["userid"] = fmt.Sprint(rand.Int31())
	ctx.HTML(200, "chat_room")
}

type messageForm struct {
	User    int    `form:"user" binding:"Required"`
	Message string `form:"message"`
}

func roomPOST(ctx *macaron.Context, message messageForm) {
	roomid := ctx.Params("roomid")
	room(roomid).Submit(fmt.Sprintf("%d: %s", message.User, message.Message))

	ctx.JSON(200, struct {
		status, message string
	}{
		status:  "success",
		message: message.Message,
	})
}

func roomDELETE(ctx *macaron.Context) {
	roomid := ctx.Params("roomid")
	deleteBroadcast(roomid)
}
