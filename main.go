package main

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"

	"path/filepath"

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

func postCDN(c *fiber.Ctx, filename string) error {
	url := os.Getenv("CDN") + "post"
	key := os.Getenv("API_KEY")

	// Open the file
	file, err := os.Open(filename)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Cannot open file: " + err.Error())
	}
	defer file.Close()

	// Create a new buffer and writer for the multipart form data
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Create the file part in the form data
	filePart, err := writer.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to create form file part: " + err.Error())
	}

	// Copy the file content into the file part
	_, err = io.Copy(filePart, file)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to copy file content: " + err.Error())
	}

	// Add other form fields
	err = writer.WriteField("filename", filename)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to add filename field: " + err.Error())
	}
	err = writer.WriteField("key", key)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to add key field: " + err.Error())
	}

	// Close the writer to finalize the form data
	writer.Close()

	// Create the HTTP request with multipart form data
	req, err := http.NewRequest("POST", url, &requestBody)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to create request: " + err.Error())
	}
	req.Header.Add("Content-Type", writer.FormDataContentType())

	// Send the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to send request to CDN: " + err.Error())
	}
	defer resp.Body.Close()

	// Check the response status
	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return c.Status(fiber.StatusInternalServerError).SendString("CDN responded with non-200 status: " + string(responseBody))
	}

	return nil
}


func getFIle(c *fiber.Ctx, filename string, filetype string) error {
	// Retrieve the file from the form input named "file"
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Failed to retrieve file: " + err.Error())
	}

	src, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to open file: " + err.Error())
	}
	defer src.Close()

	var img image.Image
	// Decode
	switch filetype {
	case "jpg", "jpeg":
		img, err = jpeg.Decode(src) // Decode for both jpg and jpeg
	case "png":
		img, err = png.Decode(src)
	case "gif":
		img, err = gif.Decode(src)
	default:
		return c.Status(fiber.StatusUnsupportedMediaType).SendString("Unsupported image type: " + filetype)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to decode image: " + err.Error())
	}

	// Define sizes for resizing
	sizes := []struct {
		width  uint
		height uint
	}{
		{800, 800}, // Large
		{300, 300}, // Medium
		{200, 200}, // Small
		{100, 100},  // Thumbnail
	}

	// Resize and save images
	for _, size := range sizes {
		resizedImg := resize.Resize(size.width, size.height, img, resize.Lanczos3)

		outputFileName := fmt.Sprintf("%s_%dx%d.%s", strings.TrimSuffix(file.Filename, filepath.Ext(file.Filename)), size.width, size.height, filetype)
		outputFile, err := os.Create(outputFileName)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to create output file: " + err.Error())
		}
		defer outputFile.Close() // Defer closing the file after writing

		// Encode the resized image based on its type
		switch filetype {
		case "jpg", "jpeg":
			err = jpeg.Encode(outputFile, resizedImg, nil)
		case "png":
			err = png.Encode(outputFile, resizedImg)
		case "gif":
			err = gif.Encode(outputFile, resizedImg, nil)
		}

		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to encode image: " + err.Error())
		}
		postCDN(c,outputFileName)
		// os.Remove(outputFileName)
	}

	return nil
}


// ForwardCDN fetches an image from the CDN, resizes it, and returns it as a PNG.
func ForwardCDN(c *fiber.Ctx, name string, width uint, height uint, quality int) error {
	// Perform a proxy forward to get the image from CDN
	filename := strings.Split(name,".")
	println(filename)
	url := os.Getenv("CDN")
	url = url + "/get"
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

	file, err := os.Create("output.jpg") // Change the filename and extension as needed
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
		return ForwardCDN(c, arr[0], uint(width), uint(height), quality)
	})

	app.Post("/proxy/post/*",func(c *fiber.Ctx) error {
		path := c.Params("*")
		filename := strings.Split(path,".")
		filetype := filename[1]
		getFIle(c,filename[0],filetype)
		return nil
	})
		// ForwardPostCDN(c,filename,filetype)
		// })

	log.Fatal(app.Listen(":5657"))
}
