package engine

import (
	"crypto/rand"
	"fmt"
	"math/big"
	mrand "math/rand/v2"
	"sync/atomic"
	"time"

	"github.com/brianvoe/gofakeit/v6"
)

var seq int64

func resolve(name string) any {
	switch name {

	// --- identidade ---
	case "uuid":
		return newUUID()
	case "seq":
		return int(atomic.AddInt64(&seq, 1))

	// --- tempo ---
	case "now":
		return time.Now().UTC().Format(time.RFC3339)
	case "now.unix":
		return time.Now().Unix()
	case "now.date":
		return time.Now().UTC().Format("2006-01-02")
	case "now.time":
		return time.Now().UTC().Format("15:04:05")

	// --- números aleatórios ---
	case "rand.int":
		return mrand.IntN(10000)
	case "rand.bool":
		return mrand.IntN(2) == 1
	case "rand.float":
		f := mrand.Float64()
		return float64(int(f*100)) / 100

	// --- pessoa ---
	case "faker.name":
		return gofakeit.Name()
	case "faker.first_name":
		return gofakeit.FirstName()
	case "faker.last_name":
		return gofakeit.LastName()
	case "faker.gender":
		return gofakeit.Gender()
	case "faker.ssn":
		return gofakeit.SSN()
	case "faker.job_title":
		return gofakeit.JobTitle()
	case "faker.job_company":
		return gofakeit.Company()
	case "faker.username":
		return gofakeit.Username()
	case "faker.password":
		return gofakeit.Password(true, true, true, false, false, 12)

	// --- contato ---
	case "faker.email":
		return gofakeit.Email()
	case "faker.phone":
		ddd := []string{"11", "21", "31", "41", "51", "61", "71", "81", "85", "91"}
		return fmt.Sprintf("+55 %s 9%04d-%04d", pick(ddd), mrand.IntN(10000), mrand.IntN(10000))
	case "faker.url":
		return gofakeit.URL()
	case "faker.domain":
		return gofakeit.DomainName()
	case "faker.ip":
		return gofakeit.IPv4Address()
	case "faker.ipv6":
		return gofakeit.IPv6Address()
	case "faker.mac":
		return gofakeit.MacAddress()
	case "faker.useragent":
		return gofakeit.UserAgent()

	// --- endereço ---
	case "faker.address":
		a := gofakeit.Address()
		return fmt.Sprintf("%s, %s", a.Street, a.City)
	case "faker.street":
		return gofakeit.Street()
	case "faker.city":
		return gofakeit.City()
	case "faker.state":
		return gofakeit.State()
	case "faker.country":
		return gofakeit.Country()
	case "faker.country_code":
		return gofakeit.CountryAbr()
	case "faker.zip":
		return gofakeit.Zip()
	case "faker.latitude":
		lat, _ := gofakeit.LatitudeInRange(-33.75, 5.27)
		return lat
	case "faker.longitude":
		lon, _ := gofakeit.LongitudeInRange(-73.98, -34.79)
		return lon

	// --- finanças ---
	case "faker.price":
		return gofakeit.Price(1, 9999)
	case "faker.currency":
		return gofakeit.CurrencyShort()
	case "faker.credit_card":
		return gofakeit.CreditCardNumber(nil)
	case "faker.iban":
		return gofakeit.AchAccount()
	case "faker.bitcoin":
		return gofakeit.BitcoinAddress()

	// --- internet / tech ---
	case "faker.color":
		return gofakeit.Color()
	case "faker.hex_color":
		return gofakeit.HexColor()
	case "faker.http_method":
		return gofakeit.HTTPMethod()
	case "faker.http_status":
		return gofakeit.HTTPStatusCode()
	case "faker.mime_type":
		return gofakeit.FileExtension()
	case "faker.file_ext":
		return gofakeit.FileExtension()
	case "faker.image_url":
		w, h := (mrand.IntN(8)+1)*100, (mrand.IntN(8)+1)*100
		return fmt.Sprintf("https://picsum.photos/%d/%d", w, h)

	// --- texto ---
	case "faker.word":
		return gofakeit.Word()
	case "faker.sentence":
		return gofakeit.Sentence(mrand.IntN(5) + 4)
	case "faker.paragraph":
		return gofakeit.Paragraph(1, 3, 8, " ")
	case "faker.lorem":
		return gofakeit.LoremIpsumSentence(mrand.IntN(5) + 4)

	// --- datas ---
	case "faker.date":
		return gofakeit.Date().UTC().Format("2006-01-02")
	case "faker.date_time":
		return gofakeit.Date().UTC().Format(time.RFC3339)
	case "faker.future_date":
		return time.Now().AddDate(0, 0, mrand.IntN(365)+1).UTC().Format("2006-01-02")
	case "faker.past_date":
		return time.Now().AddDate(0, 0, -(mrand.IntN(365) + 1)).UTC().Format("2006-01-02")

	// --- IDs brasileiros ---
	case "faker.cpf":
		return fakeCPF()
	case "faker.cnpj":
		return fakeCNPJ()
	case "faker.cep":
		return fmt.Sprintf("%05d-%03d", mrand.IntN(100000), mrand.IntN(1000))

	default:
		return "[?" + name + "]"
	}
}

func newUUID() string {
	b := make([]byte, 16)
	rand.Read(b) //nolint:errcheck
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func pick(list []string) string {
	if len(list) == 0 {
		return ""
	}
	return list[mrand.IntN(len(list))]
}

// randInt64 usa crypto/rand — reservado para uso interno onde se exige entropia segura.
func randInt64(max int64) int64 {
	n, _ := rand.Int(rand.Reader, big.NewInt(max))
	return n.Int64()
}

func fakeCPF() string {
	d := make([]int, 9)
	for i := range d {
		d[i] = mrand.IntN(10)
	}
	return fmt.Sprintf("%d%d%d.%d%d%d.%d%d%d-%02d",
		d[0], d[1], d[2], d[3], d[4], d[5], d[6], d[7], d[8], mrand.IntN(100))
}

func fakeCNPJ() string {
	d := make([]int, 12)
	for i := range d {
		d[i] = mrand.IntN(10)
	}
	return fmt.Sprintf("%d%d.%d%d%d.%d%d%d/%d%d%d%d-%02d",
		d[0], d[1], d[2], d[3], d[4], d[5], d[6], d[7], d[8], d[9], d[10], d[11], mrand.IntN(100))
}
