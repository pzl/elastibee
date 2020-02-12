package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/pzl/elastibee/pkg/auth"
	"github.com/pzl/elastibee/pkg/eco"

	"github.com/pzl/tui"
	"github.com/pzl/tui/ansi"
)

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
	os.MkdirAll("archive", 0755) // nolint

	now := time.Now()
	fmt.Printf("now: %s, start: %s. Less: %t\n", now.Format("2006-01-02"), start.Format("2006-01-02"), start.Before(now.UTC().Truncate(24*time.Hour)))
	for t := start; t.Before(now.UTC().Truncate(24 * time.Hour)); t = t.AddDate(0, 0, 20) {

		fmt.Printf("fetching dates %s through %s\n", t.Format("2006-01-02"), t.AddDate(0, 0, 19).Format("2006-01-02"))
		data, err := a.GetRuntimeData(t.Format("2006-01-02"), t.AddDate(0, 0, 19).Format("2006-01-02"))
		if err != nil {
			return err
		}
		buf, err := json.Marshal(data)
		if err != nil {
			return err
		}
		if err := ioutil.WriteFile("archive/"+t.Format("20060102")+"-"+t.AddDate(0, 0, 19).Format("20060102")+".json", buf, 0644); err != nil {
			return err
		}

		if tui.IsTTY(os.Stdout.Fd()) {
			fmt.Printf("%sDates %s%s%s%s through %s%s%s%s written\n",
				ansi.Reset,
				ansi.Bold, ansi.Cyan, t.Format("2006-01-02"), ansi.Reset,
				ansi.Bold, ansi.Cyan, t.AddDate(0, 0, 19).Format("2006-01-02"), ansi.Reset,
			)
		} else {
			fmt.Printf("%s - %s\n", t.Format("2006-01-02"), t.AddDate(0, 0, 19).Format("2006-01-02"))
		}
		time.Sleep(2 * time.Minute)
	}
	fmt.Printf("archive done\n")
	return nil
}
