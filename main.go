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

	"github.com/gin-gonic/gin"
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
	r := gin.Default()

	r.POST("/audio/start", startRecording)
	r.POST("/audio/stop", stopRecording)

	r.Run(":5757")
}

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
