package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/net/html/charset"
)

type Usdt struct {
	Id        string `xml:"ID,attr"`
	Numcode   string `xml:"NumCode"`
	Charcode  string `xml:"CharCode"`
	Nominal   string `xml:"Nominal"`
	Name      string `xml:"Name"`
	Value     string `xml:"Value"`
	Vunitrate string `xml:"VunitRate"`
}

func saveRedis(p Usdt) error {
	ctx := context.Background()

	db := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	err := db.HSet(ctx, "Валюта: "+p.Charcode, "Значение: ", p.Value).Err()
	if err != nil {
		fmt.Println("Error", err)
	}

	val, err := db.HGet(ctx, "Валюта: "+p.Charcode, "Значение: ").Result()
	if err != nil {
		fmt.Println("Error-get", err)
	}
	fmt.Printf("Курс %s: %s\n", p.Charcode, val)
	return nil

}

func main() {
	timer := time.NewTicker(10 * time.Second)
	defer timer.Stop()

	filepath := "dayly.xml"
	url := "https://www.cbr-xml-daily.ru/daily.xml"
	// take()
	downloadFile(filepath, url)
	// openFile(filepath)

	for {
		downloadFile(filepath, url)
		openFile(filepath)
		<-timer.C
	}
}

func openFile(filepath string) {
	// Открываем XML-файл
	xmlFile, err := os.Open(filepath)
	if err != nil {
		fmt.Println("Ошибка открытия XML-файла:", err)
		return
	}
	defer xmlFile.Close()

	decoder := xml.NewDecoder(xmlFile)
	decoder.CharsetReader = charset.NewReaderLabel

	var currentValute *Usdt

	for {
		token, err := decoder.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			fmt.Println("Ошибка", err)
			break
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "Valute" {
				currentValute = &Usdt{}
			}
		case xml.EndElement:
			if t.Name.Local == "Valute" && currentValute != nil {
				if currentValute.Charcode == "USD" {
					// fmt.Printf("ID: %s\nNumCode: %s\nCharCode: %s\nNominal: %s\nName: %s\nValue: %s\nVunitRate: %s\n",
					// 	currentValute.Id, currentValute.Numcode, currentValute.Charcode,
					// 	currentValute.Nominal, currentValute.Name, currentValute.Value, currentValute.Vunitrate)
					currentValute.Value = strings.Replace(currentValute.Value, ",", ".", 1)
					saveRedis(*currentValute)
				}
				currentValute = nil
			}
		case xml.CharData:
			if currentValute != nil {
				text := string(t)
				switch {
				case currentValute.Numcode == "":
					currentValute.Numcode = text
				case currentValute.Charcode == "":
					currentValute.Charcode = text
				case currentValute.Nominal == "":
					currentValute.Nominal = text
				case currentValute.Name == "":
					currentValute.Name = text
				case currentValute.Value == "":
					currentValute.Value = text
				case currentValute.Vunitrate == "":
					currentValute.Vunitrate = text
				}
			}
		}
	}
}

func downloadFile(filepath string, url string) (err error) {

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Writer the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}
