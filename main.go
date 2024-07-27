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

func main() {
	_, exists := os.LookupEnv("OPENAI_KEY")

	if exists {
	} else {

		// Load the value from the .env file
		err := godotenv.Load("/home/dmytros/.dotfiles/.env")
		if err != nil {
			fmt.Println("Error loading .env file:", err)
			return
		}
	}

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"http://localhost:3000"},
		// AllowMethods:     []string{"POST", "PATCH"},
		// AllowHeaders:     []string{"Origin"},
		// ExposeHeaders:    []string{"Content-Length"},
		// AllowCredentials: true,
		// AllowOriginFunc: func(origin string) bool {
		// 	return origin == "https://github.com"
		// },
		// MaxAge: 12 * time.Hour,
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
		c.JSON(400, gin.H{"error": "Missing recorder ID"})
		return
	}

	client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))
	resp, err := client.CreateTranscription(
		context.Background(),
		openai.AudioRequest{
			Model:    openai.Whisper1,
			FilePath: filename,
		},
	)
	if err != nil {
		fmt.Printf("Transcription error: %v\n", err)
		return
	}

	// out, err := exec.Command("whisper", filename).Output()
	// if err != nil {
	// 	c.JSON(500, gin.H{"error": err})
	// }

	// c.JSON(200, gin.H{"message": string(out)})

	c.JSON(200, gin.H{"message": resp.Text})

}

func translateAudio(c *gin.Context) {
	input := c.Query("input")
	if input == "" {
		c.JSON(400, gin.H{"error": "Missing input string"})
		return
	}

	client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4o20240513,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "You are a language interpreter between Russian and English. Your task is to listen actively to the transcription of a call and render the message completely and accurately.\nIf the transcript is in English, translate it into Russian. If the transcript is in Russian, translate it into English.\nEnsure that you interpret idea for idea and meaning for meaning, rather than word for word.\nTake a step back and think step by step about how to achieve the best result possible as defined in the steps below. You have a lot of freedom to make this work well.\n1. You provide the interpretation in target language.\n2. You only output the translation string.\n3. Do not give warnings or notes; only output the requested string.\n4. Transcript may have more than 1 language. Interpret only the language that the majority of the transcript is in.\nProper names, such as company names, brand names, etc., should not be interpreted into the target language unless they are commonly accepted equivalents. This also applies to street names, and directions included in a street name, such as north, east, etc.",
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

// 	cmd := exec.Command("rm", "/home/dmytros/projects/transcription-api/rec*")
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
	err = recorder.cmd.Process.Signal(os.Interrupt)
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
