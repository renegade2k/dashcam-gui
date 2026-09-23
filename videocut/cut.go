package videocut

import (
	"fmt"
	"os/exec"
)

// ProcessCut führt den verlustfreien Schnitt via FFmpeg durch
func ProcessCut(inputPath, outputPath, startTime, stopTime string) error {
	cmd := exec.Command("ffmpeg", "-i", inputPath, "-ss", startTime, "-to", stopTime, "-c", "copy", outputPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v\nFFmpeg Output: %s", err, string(output))
	}

	return nil
}
