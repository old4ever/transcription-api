package main

import (
	// "fmt"
	// openai "github.com/sashabaranov/go-openai"

	"errors"
	"fmt"
	"io"
	"syscall"
	"time"

	// "log"
	"net/http"

	// "github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"bytes"
	"os"
	"os/exec"
	// "sync"
)

var (
	recordingCmd *exec.Cmd
	outputFile   string
)

func main() {
	_, err := GetOpenAIApiKey()
	if err != nil {
		fmt.Println("Error loading .env file:", err)
		return
	}

	r := gin.Default()

	// corsConfig := cors.DefaultConfig()
	// corsConfig.AllowOrigins = []string{"https://example.com", "http://localhost:3000"}
	// corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	// corsConfig.AllowHeaders = []string{"Content-Type", "Authorization"}
	// r.Use(cors.New(corsConfig))

	// r.POST("/upload", handleAudioUpload)

	r.POST("/audio/start", startRecording)
	r.POST("/audio/stop", stopRecording)
	r.Run(":5757")
}

func startRecording(c *gin.Context) {
	// Generate a unique filename based on the current timestamp
	outputFile = fmt.Sprintf("recording_%d.wav", time.Now().Unix())

	// Prepare the command to execute pw-record
	recordingCmd = exec.Command("pw-record", "--channel-map", "mono", "--rate", "44000", "-P", "{ stream.capture.sink=true }", "--volume", "60.0", outputFile)

	// Set up pipes for stdout and stderr
	recordingCmd.Stdout = os.Stdout
	recordingCmd.Stderr = os.Stderr

	// Start the recording process
	err := recordingCmd.Start()
	if err != nil {
		fmt.Printf("failed to start recording: %v\n", err)
	}

	fmt.Printf("Recording started. Output file: %s\n", outputFile)
}

func stopRecording(c *gin.Context) {
	if recordingCmd == nil || recordingCmd.Process == nil {
		fmt.Println("No active recording process")
		return
	}

	// Send a SIGTERM signal to stop the recording process
	err := recordingCmd.Process.Signal(syscall.SIGTERM)
	if err != nil {
		fmt.Printf("Failed to send termination signal: %v\n", err)
		return
	}

	// Give the process a moment to clean up
	time.Sleep(100 * time.Millisecond)

	// Wait for the process to finish with a timeout
	done := make(chan error, 1)
	go func() {
		done <- recordingCmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				if exitErr.ExitCode() == 1 {
					fmt.Println("Recording process terminated as expected")
				} else {
					fmt.Printf("Recording process exited with unexpected error: %v\n", err)
				}
			} else {
				fmt.Printf("Error waiting for recording process: %v\n", err)
			}
		}
	case <-time.After(2 * time.Second):
		// If the process doesn't exit within 2 seconds, force kill it
		recordingCmd.Process.Kill()
		fmt.Println("Recording process did not exit in time, forcefully terminated")
	}

	fmt.Println("Recording stopped")

	// Check if the output file exists
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		fmt.Printf("Warning: Recording file was not created: %s\n", outputFile)
	} else {
		fmt.Printf("Recording saved to: %s\n", outputFile)
	}
}

func handleAudioUpload(c *gin.Context) {
	// Read the audio blob from the request body
	audioBlob, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Create a temporary file to store the audio blob
	tempFile, err := os.CreateTemp("", "audio-*.wav")
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFile.Name())

	// Write the audio blob to the temporary file
	_, err = io.Copy(tempFile, bytes.NewReader(audioBlob))
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Call the Unix command with the temporary file path
	var buffer bytes.Buffer
	cmd := exec.Command("echo", tempFile.Name())
	cmd.Stdout = &buffer
	err = cmd.Run()
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": buffer.String(),
	})
}

func transcribe(file string) {

	// c := openai.NewClient(os.Getenv("OPENAI_KEY"))
	// ctx := context.Background()

	// req := openai.AudioRequest{
	// 	Model:    openai.Whisper1,
	// 	FilePath: os.Args[1],
	// }
	// resp, err := c.CreateTranscription(ctx, req)
	// if err != nil {
	// 	fmt.Printf("Transcription error: %v\n", err)
	// 	return
	// }
	// fmt.Println(resp.Text)
}

func GetOpenAIApiKey() (string, error) {
	// Check if the environment variable exists
	variable, exists := os.LookupEnv("OPENAI_API_KEY")

	if exists {
		return variable, nil
	} else {
		return "", errors.New("'OPENAI_API_KEY' environment variable is not defined")
	}

}
