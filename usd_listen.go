package usd_listen

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/html/charset"
)

// Структура для валюты
type Valute struct {
	CharCode string `xml:"CharCode"`
	Value    string `xml:"Value"`
}

// Структура для списка валют
type ValCurs struct {
	Valutes []Valute `xml:"Valute"`
}

var redisClient *redis.Client

// Инициализация Redis
func init() {
	redisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
}

// Обработчик запроса курса валюты
func getExchangeRate(ctx *fasthttp.RequestCtx) {
	code := strings.ToUpper(string(ctx.QueryArgs().Peek("code"))) // Получаем код валюты

	if code == "" {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetBodyString("Ошибка: укажите код валюты в параметре 'code'")
		return
	}

	// Достаем курс из Redis
	val, err := redisClient.HGet(context.Background(), "Валюта: "+code, "Значение").Result()
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusNotFound)
		ctx.SetBodyString(fmt.Sprintf("Ошибка: курс для валюты %s не найден", code))
		return
	}

	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetBodyString(fmt.Sprintf("Курс %s: %s", code, val))
}

// Обновление курсов валют
func updateRates() {
	timer := time.NewTicker(1 * time.Hour)
	defer timer.Stop()

	filepath := "daily.xml"
	url := "https://www.cbr-xml-daily.ru/daily.xml"

	for {
		err := downloadFile(filepath, url)
		if err != nil {
			fmt.Println("Ошибка загрузки XML:", err)
			continue
		}

		err = parseXML(filepath)
		if err != nil {
			fmt.Println("Ошибка обработки XML:", err)
			continue
		}

		fmt.Println("Курсы валют успешно обновлены в Redis")
		<-timer.C
	}
}

// Загрузка XML-файла с курсами
func downloadFile(filepath, url string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	resp := fasthttp.AcquireResponse()
	req := fasthttp.AcquireRequest()
	req.SetRequestURI(url)

	err = fasthttp.Do(req, resp)
	if err != nil {
		return err
	}

	_, err = out.Write(resp.Body())
	return err
}

// Разбор XML-файла и запись в Redis
func parseXML(filepath string) error {
	xmlFile, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer xmlFile.Close()

	decoder := xml.NewDecoder(xmlFile)
	decoder.CharsetReader = charset.NewReaderLabel

	var valCurs ValCurs
	err = decoder.Decode(&valCurs)
	if err != nil {
		return err
	}

	ctx := context.Background()

	for _, valute := range valCurs.Valutes {
		valute.Value = strings.Replace(valute.Value, ",", ".", 1) // Исправляем формат
		err := redisClient.HSet(ctx, "Валюта: "+valute.CharCode, "Значение", valute.Value).Err()
		if err != nil {
			fmt.Println("Ошибка сохранения в Redis:", err)
		}
	}

	return nil
}

func main() {
	go func() {
		// Запускаем сервер с FastHTTP
		fasthttp.ListenAndServe(":8080", getExchangeRate)
		fmt.Println("Сервер запущен на порту 8080")
	}()

	updateRates()
}
