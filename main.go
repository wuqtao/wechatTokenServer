package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/valyala/fasthttp"
	"log"
	"os"
	"strconv"
	"time"
	"wechatman/config"
	"wechatman/wechat"
)
var configFile string
var test bool

func init(){
	flag.StringVar(&configFile,"conf","./wechatman.toml","assign the config file path")
	flag.BoolVar(&test,"test",false,"is test config file")

	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

func main(){
	flag.Parse()
	conf,err := config.LoadConfig(configFile)
	if err != nil{
		log.Panicln("config file error:"+err.Error())
	}

	file,err := os.Open(conf.LogFile)
	if err != nil{
		log.Println("open log file error "+err.Error())
	}else{
		log.SetOutput(file)
	}

	config.GetConfigMan().SetConfig(conf)
	//测试配置文件成功
	if test{
		fmt.Println("config file is right")
		return
	}

	wechatman,err := wechat.BuildWechatMan(conf.GetAheadTime(),conf.GetLoopTime(),conf.GetWechatConfigs()...)
	if err != nil{
		log.Panicln("get wehcatman error "+err.Error())
	}
	err = wechatman.Run()
	if err != nil{
		log.Panicln("wechatman run error "+err.Error())
	}

	err = fasthttp.ListenAndServe(":"+strconv.Itoa(conf.GetPort()),requesthandler)
	if err != nil{
		log.Panicln("fasthttp error "+err.Error())
		return
	}
}

type Result struct{
	AccessToken string `json:"accessToken"`
	Msg string         `json:"msg"`
	ServerTime int64   `json:"serverTime"`
	ExpireAt   int64   `json:"expireAt"`
}

func requesthandler(ctx *fasthttp.RequestCtx){
	log.Println(ctx.RemoteIP().String()+" request "+ctx.URI().String())
	if !ctx.IsGet(){
		ctx.Response.SetBody([]byte("only get supported"))
		return
	}
	switch(string(ctx.Path())){
		case "/query":
			if !ctx.QueryArgs().Has("appid") || !ctx.QueryArgs().Has("token"){
				ctx.Response.SetBody([]byte("param not enough"))
				return
			}

			if !QueryIpAuth(ctx.RemoteIP().String()){
				ctx.Response.SetBody([]byte("ip not in white list"))
				return
			}

			appid := ctx.QueryArgs().Peek("appid")
			token := ctx.QueryArgs().Peek("token")

			result := Result{
				ServerTime:time.Now().Unix(),
			}

			wechatman,err := wechat.GetWechatMan()
			if err != nil{
				log.Panicln("get wechatman error "+err.Error())
			}
			accessToken,expireAt,err := wechatman.QueryAccessToken(string(appid),string(token))
			if err != nil{
				log.Println(string(appid)+"query accesstoken error "+err.Error())
				result.Msg = err.Error()
			}else{
				log.Println(string(appid)+"query accesstoken success")
				result.Msg = "success"
			}

			result.ExpireAt = expireAt
			result.AccessToken = accessToken
			res,err := json.Marshal(result)
			if err !=nil{
				log.Panicln("marshal error"+err.Error())
				ctx.Response.SetBody([]byte(err.Error()))
			}else{
				ctx.Response.SetBody(res)
			}

			break
		case "/update":
			if !ctx.QueryArgs().Has("appid") || !ctx.QueryArgs().Has("token"){
				ctx.Response.SetBody([]byte("param not enough"))
				return
			}

			if !QueryIpAuth(ctx.RemoteIP().String()){
				ctx.Response.SetBody([]byte("ip not in white list"))
				return
			}

			appid := ctx.QueryArgs().Peek("appid")
			token := ctx.QueryArgs().Peek("token")

			wechatman,err := wechat.GetWechatMan()
			if err != nil{
				log.Panicln("get wechatman error "+err.Error())
				return
			}
			_,_,err = wechatman.QueryAccessToken(string(appid),string(token))
			if err != nil{
				log.Println(string(appid)+"update accesstoken error "+err.Error())
				ctx.Response.SetBody([]byte("{\"msg\":\""+err.Error()+"\"}"))
				return
			}
			log.Println(string(appid)+"update success")
			wechatman.ForceRefreshAccessToken(string(appid))
			ctx.Response.SetBody([]byte("{\"msg\":\"success\"}"))

			break
	case "/reload":
		if !ctx.QueryArgs().Has("token"){
			ctx.Response.SetBody([]byte("param not enough"))
			return
		}

		if !ReloadIpAuth(ctx.RemoteIP().String()){
			ctx.Response.SetBody([]byte("ip not in white list"))
			return
		}
		token := string(ctx.QueryArgs().Peek("token"))
		adminToken := config.GetConfigMan().GetConfig().GetAdminToken()
		if string(token) != adminToken{
			ctx.Response.SetBody([]byte("token error"))
			return
		}

		conf,err := config.LoadConfig(configFile)
		if err != nil{
			ctx.Response.SetBody([]byte(err.Error()))
			return
		}

		config.GetConfigMan().SetConfig(conf)
		wechatMan,err := wechat.GetWechatMan()
		if err != nil {
			log.Panicln("get wechatman error "+err.Error())
			ctx.Response.SetBody([]byte(err.Error()))
			return
		}

		go func (){
			err = wechatMan.Rebuild(conf.GetAheadTime(),conf.GetLoopTime(),conf.GetWechatConfigs()...)
			if err != nil{
				log.Panicln("rebuild error "+err.Error())
			}
			log.Println("reload success")
		}()
		ctx.Response.SetBody([]byte("config is reloading"))
		break
	default:
			ctx.Response.SetBody([]byte("no this route"))
	}
}

func QueryIpAuth(ip string) bool{
	conf := config.GetConfigMan().GetConfig()
	conf.RLock()
	if !conf.UseIpWhiteList{
		return true
	}
	iplist := conf.GetIpList()
	for _,ipSet := range iplist{
		if ipSet == ip {
			conf.RUnlock()
			return true
		}
	}
	conf.RUnlock()
	log.Println(ip+"not in ip list")
	return false
}

func ReloadIpAuth(ip string) bool{
	conf := config.GetConfigMan().GetConfig()
	conf.RLock()
	iplist := conf.GetAdminIpList()
	for _,ipSet := range iplist{
		if ipSet == ip {
			conf.RUnlock()
			return true
		}
	}
	conf.RUnlock()
	log.Println(ip+"not in admin ip list")
	return false
}

