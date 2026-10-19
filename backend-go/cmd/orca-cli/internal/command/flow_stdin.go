package command

import "io"
import "os"

// readAllStdin is a seam kept apart so commands that read a body or a password from a pipe share one place.
func readAllStdin() ([]byte, error) { return io.ReadAll(os.Stdin) }
