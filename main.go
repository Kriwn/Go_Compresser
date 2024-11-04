package main

import (
	"bytes"
	"image"
	"image/jpeg"

	"log"
	"os"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/proxy"
	"github.com/joho/godotenv"
	"github.com/nfnt/resize"
	"github.com/valyala/fasthttp"
)

// ForwardCDN fetches an image from the CDN, resizes it, and returns it as a PNG.
func ForwardCDN(c *fiber.Ctx, name string, width uint, height uint, quality int) error {
	// Perform a proxy forward to get the image from CDN
	url := os.Getenv("CDN")
	err := proxy.Forward(url+name, &fasthttp.Client{
		NoDefaultUserAgentHeader: true,
		DisablePathNormalizing:   true,
	})(c)

	if err != nil {
		return c.Status(505).SendString("Failed to forward request")
	}

	img, _, err := image.Decode(bytes.NewReader(c.Response().Body()))
	if err != nil {
		return c.Status(500).SendString("Failed to decode image")
	}

	if (width > 0 && height > 0){
		img = resize.Resize(width, height , img, resize.Lanczos3)
	}

	file, err := os.Create("output2.jpg") // Change the filename and extension as needed
	if err != nil {
		return c.Status(500).SendString("Failed to create output file")
	}
	defer file.Close()

	// Encode the image to the file
	err = jpeg.Encode(file, img, &jpeg.Options{Quality: quality}) // Use png.Encode if saving as PNG
	if err != nil {
		return c.Status(500).SendString("Failed to encode image")
	}


	var buf bytes.Buffer

	// Encode the image to the buffer
	err = jpeg.Encode(&buf, img, nil) // Use png.Encode(&buf, img) if saving as PNG
	if err != nil {
		return c.Status(500).SendString("Failed to encode image")
	}

	// Set the appropriate content type based on the image format
	c.Response().Header.Set("Content-Type", "image/jpeg") // Change to "image/png" if using PNG
	c.Response().SetBody(buf.Bytes())

	os.Remove("output.jpg")
	return nil
}

func main() {
	app := fiber.New()
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Failed to load environment variables")
	}

	// Route to handle image compression
	app.Get("/proxy/get/*", func(c *fiber.Ctx) error {
		path := c.Params("*")
		arr := strings.Split(path, "_")


		var width, height int
		quality := 100
		if len(arr) != 2 {
			width = 0
			height = 0
		} else {
			dimensions := strings.Split(arr[1], "*")
			if len(dimensions) < 2 {
				return c.Status(400).SendString("Invalid dimensions format")
			}

			width, err = strconv.Atoi(dimensions[0])
			if err != nil {
				return c.Status(400).SendString("Invalid width format")
			}

			height, err = strconv.Atoi(dimensions[1])
			if err != nil {
				return c.Status(400).SendString("Invalid height format")
			}

			if len(dimensions) == 3 {
				quality, err = strconv.Atoi(dimensions[2])
				if err != nil {
					return c.Status(400).SendString("Invalid height format")
				}
			}

		}
		println(quality)
		return ForwardCDN(c, arr[0], uint(width), uint(height), quality)
	})

	log.Fatal(app.Listen(":5657"))
}
