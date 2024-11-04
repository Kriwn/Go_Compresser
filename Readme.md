# Image Proxy and CDN Uploader

This application is a simple image proxy and uploader that interfaces with a CDN (Content Delivery Network) to manage image uploads and fetch resized images. Built using the Go programming language and the Fiber web framework, it supports multiple image formats and provides functionalities for resizing and posting images.

## Features

- **Image Uploading**: Users can upload images to the CDN via a POST request.
- **Image Resizing**: Images fetched from the CDN can be resized to specified dimensions.
- **Multiple Formats Supported**: The application supports JPG, PNG, and GIF formats.
- **Dynamic Quality Settings**: Users can specify the quality of the output images when resizing.

## Docker
	docker build -t compressor-go:lastest .

