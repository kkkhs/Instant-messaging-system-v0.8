package main

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

type Server struct {
	Ip   string
	Port int

	//在线用户列表
	OnlineMap map[string]*User
	mapLock   sync.RWMutex

	//消息广播的channel
	Message chan string
}

// 创建一个server的接口
func NewServer(ip string, port int) *Server {
	server := &Server{
		Ip:        ip,
		Port:      port,
		OnlineMap: make(map[string]*User),
		Message:   make(chan string),
	}

	return server
}

// 监听Message广播消息channel的goroutine,一旦有消息就发送给全部在线的User
func (this *Server) ListenMessage() {
	for {
		msg := <-this.Message

		//将msg发送给全部的User
		this.mapLock.Lock()
		for _, cli := range this.OnlineMap {
			cli.C <- msg
		}
		this.mapLock.Unlock()
	}
}

// 广播消息的方法
func (this *Server) BroadCast(user *User, msg string) {
	sendMsg := "[" + user.Addr + "]" + user.Name + ":" + msg

	this.Message <- sendMsg
}
func (this *Server) Handler(conn net.Conn) {
	//当前连接的业务
	//fmt.Println("连接建立成功")

	user := NerUser(conn, this)

	user.Online()

	//监听用户是否活跃的channel
	isLive := make(chan bool)

	//接受客户端发送的消息
	go func() {
		buf := make([]byte, 4096)
		for {
			//n为提取的长度
			n, err := conn.Read(buf)
			if n == 0 {
				user.Offline()
				return
			}

			if err != nil && err != io.EOF {
				fmt.Println("Conn Read err:", err)
				return
			}

			//读取成功：
			//提取用户的消息，去除\n
			msg := string(buf[:n-1])

			//用户针对msg进行消息处理
			user.DoMessage(msg)

			//用户的任意消息,代表当前的用户是活跃的
			isLive <- true
		}
	}()

	//当前handler阻塞
	for {
		select {
		case <-isLive:
			//当前用户是活跃的，应该重置定时器
			//不做任何事情，为了激活select,更新下面的定时器

		case <-time.After(time.Minute * 1):
			//已经超时
			//将当前的User强制关闭

			user.SendMsg("You have been kicked!")

			//销毁用户的资源
			close(user.C)

			//关闭连接
			conn.Close()

			//退出当前的Handler
			return
		}
	}

}

// 启动服务器的接口
func (this *Server) Start() {
	//1、socket listen
	listen, err := net.Listen("tcp", fmt.Sprintf("%s:%d", this.Ip, this.Port))
	if err != nil {
		fmt.Println("net.Listen err:", err)
	}
	//2、close listen socket
	defer listen.Close()

	//启动监听Message的goroutine
	go this.ListenMessage()

	for {
		//3、accept
		conn, err := listen.Accept()
		if err != nil {
			fmt.Println("listen.Accept err:", err)
			continue
		}

		//4、do handler
		go this.Handler(conn)
	}
}
