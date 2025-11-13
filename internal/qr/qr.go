package qr

import (
    qrterminal "github.com/mdp/qrterminal/v3"
    "os"
)

// Print renders the given URL as a QR code in terminal (stdout).
func Print(url string) {
    // Use a medium level and a reasonable size for typical terminals
    cfg := qrterminal.Config{
        Level:     qrterminal.M,
        Writer:    os.Stdout,
        BlackChar: qrterminal.BLACK, // full block
        WhiteChar: qrterminal.WHITE,
        QuietZone: 1,
    }
    qrterminal.GenerateWithConfig(url, cfg)
}
