/**
 * @Author Herb
 * @Date 2023/8/14 10:07
 **/

package main

import (
	"codeframework/pkg/middleware"
	"codeframework/router"
	"github.com/gin-gonic/gin"
)

func main() {

	r := gin.New()
	middleware.LoadMiddleware(r) //加载中间件

	router.LoadRoutes(r) //加载路由
	r.Run()              //启动服务

}
