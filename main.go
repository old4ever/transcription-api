package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"context"
	openai "github.com/sashabaranov/go-openai"
)

type Recorder struct {
	cmd     *exec.Cmd
	outFile string
}

var (
	recorders = make(map[int]*Recorder)
	mutex     = &sync.Mutex{}
)

type Language string

const (
	English Language = "en"
	Russian Language = "ru"
)

func (l Language) IsValid() bool {
	return l == English || l == Russian
}

func main() {
	_, exists := os.LookupEnv("OPENAI_WHISPER_API_KEY")

	if exists {
	} else {

		// Load the value from the .env file
		err := godotenv.Load(".env")
		if err != nil {
			fmt.Println("Error loading local .env file:", err)
			return
		}
	}

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"http://localhost:3000"},
	}))

	r.POST("/audio/start", startRecording)
	r.POST("/audio/stop", stopRecording)
	r.POST("/audio/transcribe", transcribeAudio)
	r.POST("/audio/translate", translateAudio)
	// r.POST("/audio/clean", removeTempFiles)

	r.Run(":5757")
}

func transcribeAudio(c *gin.Context) {
	filename := c.Query("filename")
	if filename == "" {
		c.JSON(400, gin.H{"error": "Missing filename"})
		return
	}

	client := openai.NewClient(os.Getenv("OPENAI_WHISPER_API_KEY"))

	// Retrieve the optional 'lang' parameter
	langParam := c.Query("lang")
	var lang Language

	if langParam != "" {
		lang = Language(langParam)
		if !lang.IsValid() {
			// If the language is invalid, we simply ignore it and do not set it in the request
			lang = "" // Reset lang to an empty string
		}
	}

	// Create the AudioRequest with the optional language parameter
	audioRequest := openai.AudioRequest{
		Model:    openai.Whisper1,
		FilePath: filename,
	}

	// Include the language parameter if it is valid
	if lang.IsValid() {
		audioRequest.Language = string(lang) // Assuming Language is the correct field in AudioRequest
	}

	resp, err := client.CreateTranscription(context.Background(), audioRequest)
	if err != nil {
		fmt.Printf("Transcription error: %v\n", err)
		c.JSON(500, gin.H{"error": "Transcription failed"})
		return
	}

	c.JSON(200, gin.H{"message": resp.Text})
}

func translateAudio(c *gin.Context) {
	input := c.Query("input")
	if input == "" {
		c.JSON(400, gin.H{"error": "Missing input string"})
		return
	}

	prompt := c.Query("prompt")
	// If prompt query param isn't set, it will return an empty string,
	// which is acceptable as a default "no prompt" behaviour.

	client := openai.NewClient(os.Getenv("OPENAI_TRANSCRIBE_API_KEY"))
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4o20240513,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: prompt,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: input,
				},
			},
		},
	)

	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return
	}

	// fmt.Println(resp.Choices[0].Message.Content)

	c.JSON(200, gin.H{"message": resp.Choices[0].Message.Content})
}

// TODO: FIX, it doesn't work
// Probably because shell doesn't expand '*'
// func removeTempFiles(c *gin.Context) {
// 	// cmd1 := exec.Command("killall", "pw-record")
// 	// err1 := cmd1.Start()
// 	// if err1 != nil {
// 	// 	c.JSON(500, gin.H{"error": "Failed to stop recording"})
// 	// 	return
// 	// }
// 	cmd := exec.Command("rm", "./rec*")
// 	err := cmd.Start()
// 	if err != nil {
// 		c.JSON(500, gin.H{"error": "Failed to delete files"})
// 		return
// 	}
// 	c.JSON(200, gin.H{"message": "Audio files deleted"})
// }

func startRecording(c *gin.Context) {
	// Generate a unique filename using timestamp and a random number
	timestamp := time.Now().UnixNano()
	randomPart := rand.Intn(10000)
	outFile := fmt.Sprintf("recording_%d_%d.wav", timestamp, randomPart)

	cmd := exec.Command("pw-record", "--channel-map", "mono", "--rate", "44000", "-P", "{ stream.capture.sink=true }", "--volume", "60.0", outFile)
	err := cmd.Start()
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to start recording"})
		return
	}

	pid := cmd.Process.Pid

	mutex.Lock()
	recorders[pid] = &Recorder{cmd: cmd, outFile: outFile}
	mutex.Unlock()

	c.JSON(200, gin.H{"id": pid, "message": "Recording started", "file": outFile})
}

func stopRecording(c *gin.Context) {
	idStr := c.Query("id")
	if idStr == "" {
		c.JSON(400, gin.H{"error": "Missing recorder ID"})
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid recorder ID"})
		return
	}

	mutex.Lock()
	recorder, exists := recorders[id]
	mutex.Unlock()

	if !exists {
		c.JSON(404, gin.H{"error": "Recorder not found. No recording under this pid?"})
		return
	}

	// Send SIGINT to the process
	err = recorder.cmd.Process.Signal(os.Kill)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to stop recording"})
		return
	}

	// Wait for the process to exit
	err = recorder.cmd.Wait()
	if err != nil {
		// Check if the error is due to a non-zero exit status
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				// pw-record returns exit code 1 when interrupted
				if status.ExitStatus() != 1 {
					c.JSON(500, gin.H{"error": "Unexpected error while stopping recording"})
					return
				}
				// If it's 1, we consider it a normal termination due to our interrupt
			}
		} else {
			// If it's not an ExitError, it's unexpected
			c.JSON(500, gin.H{"error": "Unexpected error while waiting for recording to stop"})
			return
		}
	}

	mutex.Lock()
	delete(recorders, id)
	mutex.Unlock()

	c.JSON(200, gin.H{"message": "Recording stopped", "file": recorder.outFile})
}
