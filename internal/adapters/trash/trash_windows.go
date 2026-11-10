package trash

import "errors"

// move has no Recycle Bin to reach without the shell's COM interface, so on
// Windows an existing file stays where it is and the caller picks another name.
func move(string) (string, error) {
	return "", errors.New("codeshot cannot reach the Recycle Bin on Windows yet")
}
