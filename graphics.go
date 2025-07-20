package main

import (
	"image"
	"image/color"
	"image/draw"
	"log"
	"time"

	"golang.org/x/exp/shiny/driver"
	"golang.org/x/exp/shiny/screen"
	"golang.org/x/mobile/event/lifecycle"
)

const SCREEN_WIDTH = 1600
const SCREEN_HEIGHT = 900
const SCREEN_TITLE = "GoCart"
const FPS = 60

func draw_loop(controllers []*Controller, exit_channel chan struct{}) {
	driver.Main(func(s screen.Screen) {
		w, err := s.NewWindow(&screen.NewWindowOptions{
			Title:  "Simple Rectangle",
			Width:  SCREEN_WIDTH,
			Height: SCREEN_HEIGHT,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer w.Release()

		// Create an RGBA buffer
		buffer, err := s.NewBuffer(image.Point{X: SCREEN_WIDTH, Y: SCREEN_HEIGHT})
		if err != nil {
			log.Fatal(err)
		}
		defer buffer.Release()

		ticker := time.NewTicker(time.Second / FPS)
		defer ticker.Stop()

		go func() {
			for range ticker.C {

				// Fill background white
				draw.Draw(buffer.RGBA(), buffer.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

				// Draw a blue rectangle at each cart's position
				for _, controller := range controllers {
					cart := controller.Cart

					// Pick the cart's color based the controller's state
					var cartColor color.RGBA
					switch controller.State {
					case Idle:
						cartColor = color.RGBA{0, 0, 255, 255} // Blue
					case Processing:
						cartColor = color.RGBA{255, 255, 0, 255} // Yellow
					case Requesting:
						cartColor = color.RGBA{255, 165, 0, 255} // Orange
					case Moving:
						cartColor = color.RGBA{0, 255, 0, 255} // Green
					case Avoiding:
						cartColor = color.RGBA{255, 0, 0, 255} // Red
					}

					// Draw a rectangle representing the cart
					rect := image.Rect(int(cart.Position-cart.Width/2), SCREEN_HEIGHT/2-int(cart.Height)/2, int(cart.Position+cart.Width/2), SCREEN_HEIGHT/2+int(cart.Height)/2)
					draw.Draw(buffer.RGBA(), rect, &image.Uniform{cartColor}, image.Point{}, draw.Src)

					// Draw the goal
					goalRect := image.Rect(int(controller.Goal-cart.Width/20), SCREEN_HEIGHT/2-int(cart.Height)/20, int(controller.Goal+cart.Width/20), SCREEN_HEIGHT/2+int(cart.Height)/20)
					draw.Draw(buffer.RGBA(), goalRect, &image.Uniform{color.RGBA{255, 0, 0, 255}}, image.Point{}, draw.Src)

					// Draw the position setpoint
					setpointRect := image.Rect(int(controller.PositionPID.Setpoint-cart.Width/20), SCREEN_HEIGHT/2-int(cart.Height)/20, int(controller.PositionPID.Setpoint+cart.Width/20), SCREEN_HEIGHT/2+int(cart.Height)/20)
					draw.Draw(buffer.RGBA(), setpointRect, &image.Uniform{color.RGBA{0, 0, 0, 255}}, image.Point{}, draw.Src)

					// Draw the left and right bounds as single pixel lines from top to bottom
					leftBound := image.Rect(int(controller.LeftBound), 0, int(controller.LeftBound)+2, SCREEN_HEIGHT)
					draw.Draw(buffer.RGBA(), leftBound, &image.Uniform{color.RGBA{0, 255, 0, 255}}, image.Point{}, draw.Src)
					rightBound := image.Rect(int(controller.RightBound), 0, int(controller.RightBound)-2, SCREEN_HEIGHT)
					draw.Draw(buffer.RGBA(), rightBound, &image.Uniform{color.RGBA{0, 255, 0, 255}}, image.Point{}, draw.Src)
				}

				// Upload to the window
				w.Upload(image.Point{}, buffer, buffer.Bounds())
				w.Publish()
			}
		}()

		// Check for window events
		for {
			switch e := w.NextEvent().(type) {
			case lifecycle.Event:
				if e.To == lifecycle.StageDead {
					exit_channel <- struct{}{}
					return
				}
			}
		}
	})
}
