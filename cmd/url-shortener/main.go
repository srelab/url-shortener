package main

import (
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/srelab/url-shortener/pkg"

	"github.com/srelab/url-shortener/pkg/g"
	"github.com/srelab/url-shortener/pkg/logger"
	"github.com/srelab/url-shortener/pkg/util"
	"github.com/urfave/cli"
)

func main() {
	app := &cli.App{
		Name:     g.NAME,
		Usage:    "URL Shortening Service",
		Version:  g.VERSION,
		Compiled: time.Now(),
		Authors:  []cli.Author{{Name: g.AUTHOR, Email: g.MAIL}},
		Before: func(c *cli.Context) error {
			fmt.Fprintf(c.App.Writer, util.StripIndent(
				`
				#     # ######  #           #####  #     # ####### ######  ####### 
				#     # #     # #          #     # #     # #     # #     #    #    
				#     # #     # #          #       #     # #     # #     #    #    
				#     # ######  #           #####  ####### #     # ######     #    
				#     # #   #   #                # #     # #     # #   #      #    
				#     # #    #  #          #     # #     # #     # #    #     #    
 				#####  #     # #######     #####  #     # ####### #     #    #
			`))
			return nil
		},
		Commands: []cli.Command{
			{
				Name:  "start",
				Usage: "start a new gateway-register",
				Action: func(ctx *cli.Context) {
					for _, flagName := range ctx.FlagNames() {
						if ctx.String(flagName) != "" {
							continue
						}

						fmt.Println(flagName + " is required")
						os.Exit(127)
					}

					if err := g.ReadInConfig(ctx); err != nil {
						logger.Fatal("could not read config: %v", err)
					}
					logger.InitLogger()

					canStop := make(chan os.Signal, 1)
					signal.Notify(canStop, os.Interrupt)

					stop, err := pkg.Start()
					if err != nil {
						logger.Fatal("could not init shortener: %v", err)
					}

					<-canStop
					logger.Error("Shutting down...")
					stop()
				},
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "config, c", Usage: "Load configuration from `FILE`"},
				},
			},
		},
	}

	app.Run(os.Args)
}
