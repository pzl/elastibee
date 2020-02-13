package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pzl/elastibee/pkg/auth"
	"github.com/pzl/elastibee/pkg/eco"
	"github.com/pzl/elastibee/pkg/elastic"

	"github.com/pzl/tui"
	"github.com/pzl/tui/ansi"
)

const index = "eco"
const indexFile = "etc/mapping.json"

func pin(a *eco.App) error {
	pin, err := auth.MakePin(a.AppKey)
	if err != nil {
		return err
	}

	if tui.IsTTY(os.Stdout.Fd()) {
		fmt.Printf("%sPlease visit %shttps://www.ecobee.com/consumerportal/index.html#/my-apps%s and enter the Pin code: %s%s%s%s\n", ansi.Reset, ansi.Magenta, ansi.Reset, ansi.Bold, ansi.Blue, pin.Pin, ansi.Reset)
	} else {
		fmt.Printf("Pin: %s\nCode: %s", pin.Pin, pin.Code)
		return nil
	}

	if !tui.IsTTY(os.Stdin.Fd()) {
		fmt.Printf("When finished, run: %s token %s\n", os.Args[0], pin.Code)
		return nil
	}

	fmt.Print("Press enter when finished...")
	_, err = bufio.NewReader(os.Stdin).ReadBytes('\n')
	if err != nil {
		return err
	}

	// write out refresh token
	return getToken(a, pin.Code)
}

func getToken(a *eco.App, code string) error {
	tk, err := auth.MakeToken(a.AppKey, code)
	if err != nil {
		return err
	}

	if tk.AccessToken == "" || tk.Refresh == "" {
		return errors.New("unable to fetch tokens from ecobee. Possibly empty response.")
	}

	// write tokens
	a.AccessToken = tk.AccessToken
	a.RefreshToken = tk.Refresh
	return a.Save()
}

func main() {
	a, err := eco.Open()
	if err != nil {
		panic(err)
	}

	switch os.Args[1] {
	case "pin":
		if err := pin(a); err != nil {
			panic(err)
		}
	case "token":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "parameter expected: actual token text")
			os.Exit(1)
		}
		if err := getToken(a, os.Args[2]); err != nil {
			panic(err)
		}
	case "refresh":
		if err := a.Refresh(); err != nil {
			panic(err)
		}
		fmt.Println("refresh successful")
	case "archive":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "parameter expected: start date")
			os.Exit(1)
		}

		start, err := time.Parse("2006-01-02", os.Args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error parsing start date: %v", err)
			os.Exit(1)
		}
		if err := archive(a, start); err != nil {
			panic(err)
		}
	}
}

func archive(a *eco.App, start time.Time) error {
	tty := tui.IsTTY(os.Stdout.Fd())
	os.MkdirAll("archive", 0755) // nolint
	client := elastic.New("http://estc:9200")
	if !client.IndexExists(index) {
		if err := client.CreateIndexFromFile(index, indexFile); err != nil {
			return err
		}
		if tty {
			fmt.Printf("index %s%s%s%s created\n", ansi.Bold, ansi.Magenta, index, ansi.Reset)
		} else {
			fmt.Printf("index %s created\n", index)
		}
	}

	w := ansi.NewWriter(os.Stdout)
	if tty {
		w.CursorHide()
		defer w.CursorShow()
	}

	finished := fmt.Sprintf("%s%s✔%s", ansi.Bold, ansi.Green, ansi.Reset)

	now := time.Now()
	for t := start; t.Before(now.UTC().Truncate(24 * time.Hour)); t = t.AddDate(0, 0, 20) {
		done := make(chan struct{})

		if tty {
			fmt.Printf("Date range %s%s%s%s -> %s%s%s%s\n  Fetching: \n  Transforming: \n  Sending: \n  Saving: ", ansi.Cyan, ansi.Bold, t.Format("2006-01-02"), ansi.Reset, ansi.Cyan, ansi.Bold, t.AddDate(0, 0, 19).Format("2006-01-02"), ansi.Reset)
			w.Up(3)
			w.Column(14)
			go spin(done, w)
		}
		data, err := a.GetRuntimeData(t.Format("2006-01-02"), t.AddDate(0, 0, 19).Format("2006-01-02"))
		if tty {
			done <- struct{}{}
		}
		if err != nil {
			return err
		}
		if tty {
			w.Column(13)
			fmt.Print(finished)
			w.Down(1)
			w.Column(18)
			go spin(done, w)
		}
		nd, err := toNdJson(data)
		if tty {
			done <- struct{}{}
		}
		if err != nil {
			return err
		}
		if tty {
			w.Column(17)
			fmt.Print(finished)
			w.Down(1)
			w.Column(13)
			go spin(done, w)
		}

		file := "archive/" + t.Format("20060102") + "-" + t.AddDate(0, 0, 19).Format("20060102") + ".json"
		err = stream(nd, client, file)
		if tty {
			done <- struct{}{}
		}
		if err != nil {
			return err
		}
		if tty {
			w.Column(12)
			fmt.Print(finished)
			w.Down(1)
			w.Column(11)
			fmt.Print(finished)
		}
		time.Sleep(8 * time.Second)
		if tty {
			w.Up(4)
			w.Column(35)
			fmt.Print(": " + finished)
			w.ClearDown()
			w.Down(1)
			w.Column(0)
		}
	}
	fmt.Printf("archive done\n")
	return nil
}

func spin(done chan struct{}, w *ansi.Writer) {
	boxes := []rune(`⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`)
	blen := len(boxes)
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for i := 0; true; i++ {
		select {
		case <-done:
			return
		case <-tick.C:
			w.Left(1)
			fmt.Printf("%c", boxes[i%blen])
		}
	}
}

func toNdJson(data eco.RuntimeData) (io.Reader, error) {
	var buf bytes.Buffer
	for _, d := range data.Data {
		buf.WriteString("{\"index\":{}}\n")
		ln, err := json.Marshal(d)
		if err != nil {
			return nil, err
		}
		buf.Write(ln)
		buf.WriteRune('\n')
	}

	for _, d := range data.SensorData {
		buf.WriteString("{\"index\":{}}\n")
		ln, err := json.Marshal(d)
		if err != nil {
			return nil, err
		}
		buf.Write(ln)
		buf.WriteRune('\n')
	}
	return &buf, nil
}

func stream(data io.Reader, client elastic.Client, file string) error {
	f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	tee := io.TeeReader(data, f)

	if err := client.Bulk(index, tee); err != nil {
		return err
	}
	return nil
}
