// Alat uji Langkah 9 Modul 10: menjalankan seluruh skenario ulasan kelas
// secara berurutan, lalu mencetak hasil setiap request.
//
// Pemakaian (server harus sedang berjalan di terminal lain):
//
//	go run ./tools/reviewtest
//
// Alat ini mencetak token sendiri dengan JWT_SECRET (sama seperti
// tools/loadtest), karena akun mhs001-mhs060 tidak dapat dipakai login.
// Setiap kali dijalankan, ulasan kelas IF101 di MongoDB dikosongkan dulu,
// jadi alat ini aman dijalankan berulang kali.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"latihan-fiber/app/model"
	"latihan-fiber/config"
	"latihan-fiber/database"
	"latihan-fiber/helper"
)

var (
	baseURL = "http://localhost:3000/api/v1"
	client  = &http.Client{Timeout: 15 * time.Second}
	tokens  = map[string]string{}
	nomor   = 0
)

type response struct {
	Status   int
	Code     string
	Message  string
	Location string
	Data     map[string]any
	Raw      string
}

func main() {
	config.LoadEnv()
	ctx := context.Background()

	pool, err := database.NewPool(ctx)
	if err != nil {
		fail("PostgreSQL: %v", err)
	}
	defer pool.Close()

	mongoClient, err := database.NewMongoClient(ctx)
	if err != nil {
		fail("MongoDB: %v", err)
	}
	defer func() { _ = mongoClient.Disconnect(context.Background()) }()
	reviews := mongoClient.Database(config.GetEnv("MONGO_DATABASE", "praktikum_backend")).
		Collection("course_reviews")

	jwtManager := helper.NewJWTManager(
		config.GetEnv("JWT_SECRET", ""),
		config.GetEnv("JWT_ISSUER", "praktikum-backend"),
		time.Duration(config.GetEnvInt("JWT_ACCESS_TTL_MINUTES", 15))*time.Minute,
	)

	// Token untuk mhs001-mhs040 dan admin.
	rows, err := pool.Query(ctx,
		`SELECT id, username, role FROM users
		 WHERE username ~ '^mhs0[0-4][0-9]$' OR username = 'admin'`)
	if err != nil {
		fail("membaca users: %v", err)
	}
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role); err != nil {
			fail("membaca users: %v", err)
		}
		t, err := jwtManager.GenerateAccess(u)
		if err != nil {
			fail("membuat token: %v", err)
		}
		tokens[u.Username] = t
	}
	rows.Close()
	for _, need := range []string{"admin", "mhs001", "mhs002", "mhs003", "mhs040"} {
		if tokens[need] == "" {
			fail("akun %s tidak ditemukan; jalankan migrations/006_seed_loadtest.sql", need)
		}
	}

	var courseID int
	if err := pool.QueryRow(ctx, "SELECT id FROM courses WHERE code = 'IF101'").Scan(&courseID); err != nil {
		fail("kelas IF101 tidak ditemukan: %v", err)
	}
	base := fmt.Sprintf("/courses/%d/reviews", courseID)

	// ------------------------------------------------------------ persiapan
	fmt.Println("== PERSIAPAN ==")
	del, err := reviews.DeleteMany(ctx, bson.M{"course_id": courseID})
	if err != nil {
		fail("mengosongkan ulasan: %v", err)
	}
	fmt.Printf(" ulasan lama IF101 dihapus dari MongoDB: %d dokumen\n", del.DeletedCount)
	r := call("DELETE", fmt.Sprintf("/courses/%d/enroll", courseID), "mhs003", nil)
	fmt.Printf(" mhs003 dikeluarkan dari IF101       : %d %s\n", r.Status, r.Code)
	for _, u := range []string{"mhs001", "mhs002"} {
		r := call("POST", fmt.Sprintf("/courses/%d/enroll", courseID), u, nil)
		fmt.Printf(" %s didaftarkan ke IF101           : %d %s\n", u, r.Status, r.Code)
		if r.Status != 201 && r.Code != "ALREADY_ENROLLED" {
			fail("%s gagal didaftarkan ke IF101", u)
		}
	}
	fmt.Println()

	// ------------------------------------------------------------ fungsional
	fmt.Println("== UJI FUNGSIONAL ==")

	r = call("POST", base, "mhs003", map[string]any{
		"rating": 4, "comment": "Materi Go-nya jelas sekali", "tags": []string{"dosen-jelas"}})
	show("mhs003 menulis ulasan IF101 (tidak terdaftar)", "403 NOT_ENROLLED", r)

	r = call("POST", base, "mhs001", map[string]any{
		"rating": 4, "comment": "Materi Go-nya jelas, tugasnya cukup banyak",
		"tags": []string{"Tugas-Banyak", " dosen-jelas ", "tugas-banyak"}})
	show("mhs001 menulis ulasan, tag berantakan", "201, tags dirapikan, Location terisi", r)
	fmt.Printf("    tags     : %v\n    Location : %s\n", r.Data["tags"], r.Location)
	review1, _ := r.Data["id"].(string)
	created1, _ := r.Data["created_at"].(string)
	if review1 == "" {
		fail("ulasan mhs001 gagal dibuat, uji dihentikan")
	}

	r = call("POST", base, "mhs001", map[string]any{
		"rating": 5, "comment": "Ulasan kedua untuk kelas yang sama"})
	show("mhs001 menulis ulasan kedua di kelas yang sama", "409 ALREADY_REVIEWED", r)

	r = call("POST", base, "mhs002", map[string]any{
		"rating": 9, "comment": "pendek", "tags": []string{"a1", "b2", "c3", "d4", "e5", "f6"}})
	show("rating 9, komentar \"pendek\", 6 tag", "422 dengan pesan per field", r)
	fmt.Printf("    errors   : %s\n", errorsOf(r.Raw))

	r = call("POST", "/courses/99999/reviews", "mhs001", map[string]any{
		"rating": 4, "comment": "Kelas ini tidak pernah ada"})
	show("ulasan untuk kelas 99999", "404", r)

	r = call("GET", "/reviews/bukan-objectid", "mhs001", nil)
	show("GET /reviews/bukan-objectid", "400 id ulasan tidak valid", r)

	r = call("POST", base, "mhs002", map[string]any{
		"rating": 3, "comment": "Lumayan, tapi praktikumnya padat", "tags": []string{"praktikum-padat", "tugas-banyak"}})
	show("mhs002 menulis ulasan", "201", r)
	review2, _ := r.Data["id"].(string)

	r = call("GET", base, "mhs003", nil)
	show("GET daftar ulasan IF101", "200, 2 ulasan", r)
	fmt.Printf("    jumlah   : %d ulasan\n", countList(r.Raw))

	r = call("GET", base+"?tag=tugas-banyak&limit=1", "mhs003", nil)
	show("GET daftar ulasan ?tag=tugas-banyak&limit=1", "200, 1 ulasan, has_more true", r)
	fmt.Printf("    meta     : %s\n", metaOf(r.Raw))

	r = call("GET", base+"/summary", "mhs003", nil)
	show("GET ringkasan ulasan IF101", "200, count 2, rata-rata 3.5", r)
	fmt.Printf("    data     : %s\n", compact(r.Data))

	r = call("GET", "/reviews/"+review1, "mhs003", nil)
	show("GET /reviews/:id ulasan mhs001", "200", r)

	r = call("PATCH", "/reviews/"+review1, "mhs002", map[string]any{"rating": 1})
	show("mhs002 mengubah ulasan mhs001", "403", r)

	time.Sleep(20 * time.Millisecond)
	r = call("PATCH", "/reviews/"+review1, "mhs001", map[string]any{
		"rating": 5, "comment": "Setelah UTS ternyata materinya makin seru"})
	show("mhs001 mengubah ulasannya sendiri", "200, updated_at berubah, created_at tetap", r)
	fmt.Printf("    created_at sebelum : %s\n    created_at sesudah : %v\n    updated_at sesudah : %v\n",
		created1, r.Data["created_at"], r.Data["updated_at"])

	r = call("POST", "/reviews/"+review1+"/replies", "mhs003", map[string]any{"body": "Setuju, materinya bagus"})
	show("mhs003 membalas ulasan mhs001", "201, replies berisi 1", r)
	if reps, ok := r.Data["replies"].([]any); ok {
		fmt.Printf("    replies  : %d balasan\n", len(reps))
	}

	r = call("POST", "/reviews/"+review1+"/helpful", "mhs001", nil)
	show("mhs001 menilai ulasannya sendiri \"membantu\"", "409 OWN_REVIEW", r)

	r = call("POST", "/reviews/"+review1+"/helpful", "mhs002", nil)
	show("mhs002 menilai \"membantu\"", "200, helpful_count 1", r)
	fmt.Printf("    helpful_count : %v\n", r.Data["helpful_count"])

	r = call("POST", "/reviews/"+review1+"/helpful", "mhs002", nil)
	show("mhs002 menilai \"membantu\" lagi", "409 ALREADY_VOTED", r)

	// ------------------------------------------------------------ keamanan
	fmt.Println()
	fmt.Println("== UJI NoSQL INJECTION ==")
	r = callRaw("PATCH", "/reviews/"+review1, "mhs001", `{"comment": {"$ne": ""}}`)
	show("body {\"comment\": {\"$ne\": \"\"}}", "400, ditolak sebelum menyentuh database", r)

	r = call("GET", base+"?tag="+url.QueryEscape(`{"$ne":""}`), "mhs003", nil)
	show("?tag={\"$ne\":\"\"} pada daftar ulasan", "200 dengan 0 hasil", r)
	fmt.Printf("    jumlah   : %d ulasan\n", countList(r.Raw))

	// ------------------------------------------------------------ serentak
	fmt.Println()
	fmt.Println("== UJI SERENTAK ==")
	voters := []string{}
	for i := 4; i <= 33; i++ {
		voters = append(voters, fmt.Sprintf("mhs%03d", i))
	}
	tally := fire(len(voters), func(i int) string {
		return label(call("POST", "/reviews/"+review2+"/helpful", voters[i], nil))
	})
	fmt.Println(" 30 mahasiswa berbeda menilai ulasan mhs002 secara serentak:")
	printTally(tally)
	fmt.Printf("   helpful_count di MongoDB : %d (harus 30)\n", helpfulCount(ctx, reviews, review2))

	tally = fire(20, func(int) string {
		return label(call("POST", "/reviews/"+review1+"/helpful", "mhs040", nil))
	})
	fmt.Println(" 1 mahasiswa (mhs040) menilai ulasan mhs001 20 kali serentak:")
	printTally(tally)
	fmt.Printf("   helpful_count di MongoDB : %d (harus 2: mhs002 + mhs040)\n", helpfulCount(ctx, reviews, review1))

	// ------------------------------------------------------------ hapus
	fmt.Println()
	fmt.Println("== UJI HAPUS ==")
	r = call("DELETE", "/reviews/"+review2, "mhs003", nil)
	show("mhs003 menghapus ulasan mhs002", "403", r)

	r = call("DELETE", "/reviews/"+review2, "admin", nil)
	show("admin menghapus ulasan mhs002", "204", r)

	r = call("DELETE", "/reviews/"+review2, "admin", nil)
	show("menghapus ulasan yang sudah terhapus", "404", r)

	fmt.Println()
	fmt.Println("Selesai. Buka Compass -> praktikum_backend -> course_reviews untuk melihat dokumennya.")
}

// ---------------------------------------------------------------- helper

func call(method, path, user string, body any) response {
	var raw string
	if body != nil {
		b, _ := json.Marshal(body)
		raw = string(b)
	}
	return callRaw(method, path, user, raw)
}

func callRaw(method, path, user, body string) response {
	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	req, _ := http.NewRequest(method, baseURL+path, reader)
	req.Header.Set("Authorization", "Bearer "+tokens[user])
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return response{Status: 0, Code: "ERR", Message: err.Error()}
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	out := response{Status: resp.StatusCode, Location: resp.Header.Get("Location"), Raw: string(data)}
	var parsed struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Data    any    `json:"data"`
	}
	_ = json.Unmarshal(data, &parsed)
	out.Code, out.Message = parsed.Code, parsed.Message
	if m, ok := parsed.Data.(map[string]any); ok {
		out.Data = m
	} else {
		out.Data = map[string]any{}
	}
	return out
}

func show(title, expect string, r response) {
	nomor++
	fmt.Printf("\n%2d. %s\n", nomor, title)
	fmt.Printf("    harapan  : %s\n", expect)
	got := fmt.Sprintf("%d", r.Status)
	if r.Code != "" {
		got += " " + r.Code
	}
	fmt.Printf("    hasil    : %s\n", got)
	if r.Message != "" {
		fmt.Printf("    message  : %s\n", r.Message)
	}
}

func label(r response) string {
	if r.Status >= 200 && r.Status < 300 {
		return fmt.Sprintf("%d OK", r.Status)
	}
	if r.Code != "" {
		return fmt.Sprintf("%d %s", r.Status, r.Code)
	}
	return fmt.Sprintf("%d", r.Status)
}

func fire(n int, job func(i int) string) map[string]int {
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results <- job(i)
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)

	tally := map[string]int{}
	for r := range results {
		tally[r]++
	}
	return tally
}

func printTally(tally map[string]int) {
	keys := make([]string, 0, len(tally))
	for k := range tally {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("   %-26s %d\n", k, tally[k])
	}
}

func errorsOf(raw string) string {
	var p struct {
		Fields map[string]string `json:"fields"`
	}
	_ = json.Unmarshal([]byte(raw), &p)
	keys := make([]string, 0, len(p.Fields))
	for k := range p.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := []string{}
	for _, k := range keys {
		parts = append(parts, k+": "+p.Fields[k])
	}
	return strings.Join(parts, "; ")
}

// helpfulCount membaca helpful_count langsung dari MongoDB, bukan dari API,
// agar hasil uji serentak dibandingkan dengan isi database yang sebenarnya.
func helpfulCount(ctx context.Context, coll *mongo.Collection, id string) int {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return -1
	}
	var doc struct {
		HelpfulCount int `bson:"helpful_count"`
	}
	if err := coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		return -1
	}
	return doc.HelpfulCount
}

func countList(raw string) int {
	var p struct {
		Data []any `json:"data"`
	}
	_ = json.Unmarshal([]byte(raw), &p)
	return len(p.Data)
}

func metaOf(raw string) string {
	var p struct {
		Meta any `json:"meta"`
	}
	_ = json.Unmarshal([]byte(raw), &p)
	b, _ := json.Marshal(p.Meta)
	return string(b)
}

func compact(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "reviewtest: "+format+"\n", args...)
	os.Exit(1)
}
