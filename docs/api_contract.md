# API Contract — Sehatiku Backend

Dokumen ini adalah sumber kebenaran untuk seluruh endpoint API yang sudah diimplementasi.
**Setiap endpoint baru atau perubahan endpoint lama HARUS ditambahkan/diperbarui di sini**
(lihat Aturan #3 di `CLAUDE.md`).

Jangan menulis ulang seluruh dokumen saat menambah entry — tambahkan entry baru di bagian
yang sesuai, atau update entry yang sudah ada bila method+path sama.

---

## Format Entry

Gunakan format berikut untuk setiap endpoint:

````md
### `METHOD /path/ke/endpoint`

**Deskripsi:** Satu-dua kalimat tujuan endpoint ini.

**Auth:** Bearer JWT (nakes) | Bearer JWT (patient) | Public | Internal (ML microservice)

**Request**

Path params (jika ada):
| Param | Tipe | Keterangan |
|---|---|---|

Query params (jika ada):
| Param | Tipe | Wajib | Keterangan |
|---|---|---|---|

Body (jika ada):
```json
{
  "field": "tipe — keterangan singkat"
}
```

**Response — 200 OK**
```json
{
  "field": "tipe — keterangan singkat"
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | ... |
| 401 | ... |
| 404 | ... |

**Catatan:** efek samping (mis. menulis ke tabel `notifications`), batasan, atau
hal non-obvious lain yang perlu diketahui pemanggil endpoint ini.
````

---

## Daftar Endpoint

### Auth

---

### `POST /api/v1/faskes/auth/register`

**Deskripsi:** Mendaftarkan fasilitas kesehatan baru. Setelah register, faskes harus login terpisah untuk mendapat token.

**Auth:** Public

**Request**

Body:
```json
{
  "name": "string — nama faskes",
  "type": "string — puskesmas | klinik",
  "address": "string — alamat lengkap",
  "region": "string — wilayah/kota",
  "username": "string — min 4, max 50 karakter",
  "password": "string — min 8 karakter",
  "phone_number": "string — format internasional, contoh: 628123456789"
}
```

**Response — 200 OK**
```json
{
  "message": "faskes berhasil didaftarkan",
  "data": null
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | Body tidak valid / field wajib kosong / format `type` salah |
| 409 | `username` sudah digunakan faskes lain |
| 500 | Kegagalan server |

---

### `POST /api/v1/faskes/auth/login`

**Deskripsi:** Login faskes menggunakan username dan password. Mengembalikan access token (JWT, 12 jam) dan refresh token (opaque, 7 hari).

**Auth:** Public

**Request**

Body:
```json
{
  "username": "string",
  "password": "string"
}
```

**Response — 200 OK**
```json
{
  "message": "login berhasil",
  "data": {
    "token": {
      "access_token": "string — JWT HS256",
      "refresh_token": "string — opaque 32-byte base64url",
      "expires_in": 43200
    },
    "faskes_id": "string — UUID faskes",
    "name": "string — nama faskes"
  }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | Body tidak valid |
| 401 | Username/password salah, atau akun `inactive` |
| 429 | Melebihi 5 percobaan gagal dalam 15 menit |
| 500 | Kegagalan server |

**Catatan:** JWT claims: `typ="faskes"`, `faskes_id`. Setelah login berhasil, notifikasi WhatsApp dikirim ke `phone_number` faskes secara fire-and-forget via whatsmeow (kegagalan WA hanya di-log, tidak memblok response). Notifikasi hanya terkirim jika server sudah dipasangkan ke nomor WA melalui `cmd/wa-setup`.

---

### `POST /api/v1/nakes/auth/login`

**Deskripsi:** Login nakes (dokter/kader/admin) menggunakan username dan password. Akun nakes dibuat oleh admin faskes — tidak ada self-register. Multi-device diperbolehkan; sesi lama tidak direvoke.

**Auth:** Public

**Request**

Body:
```json
{
  "username": "string",
  "password": "string"
}
```

**Response — 200 OK**
```json
{
  "message": "login berhasil",
  "data": {
    "token": {
      "access_token": "string — JWT HS256",
      "refresh_token": "string — opaque 32-byte base64url",
      "expires_in": 43200
    },
    "nakes_id": "string — UUID nakes",
    "faskes_id": "string — UUID faskes tempat nakes bertugas",
    "full_name": "string",
    "role": "string — dokter | kader | admin"
  }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | Body tidak valid |
| 401 | Username/password salah, atau akun `inactive` |
| 429 | Melebihi 5 percobaan gagal dalam 15 menit |
| 500 | Kegagalan server |

**Catatan:** JWT claims: `typ="nakes"`, `nakes_id`, `faskes_id`, `role`. Refresh token berlaku 7 hari. Notifikasi WhatsApp dikirim ke `phone_number` nakes setelah login berhasil via whatsmeow (fire-and-forget; hanya terkirim jika server sudah dipasangkan via `cmd/wa-setup`).

---

### `POST /api/v1/patients/auth/login`

**Deskripsi:** Login pasien menggunakan username dan password. Akun pasien dibuat oleh nakes — tidak ada self-register. Single-device policy: login baru otomatis merevoke seluruh sesi lama.

**Auth:** Public

**Request**

Body:
```json
{
  "username": "string",
  "password": "string"
}
```

**Response — 200 OK**
```json
{
  "message": "login berhasil",
  "data": {
    "token": {
      "access_token": "string — JWT HS256",
      "refresh_token": "string — opaque 32-byte base64url",
      "expires_in": 43200
    },
    "patient_id": "string — UUID pasien",
    "faskes_id": "string — UUID faskes yang mendaftarkan pasien",
    "full_name": "string"
  }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | Body tidak valid |
| 401 | Username/password salah, atau akun `inactive` |
| 429 | Melebihi 5 percobaan gagal dalam 15 menit |
| 500 | Kegagalan server |

**Catatan:** JWT claims: `typ="patient"`, `patient_id`, `faskes_id`. Refresh token berlaku 60 hari (UX lansia). Sebelum token baru diterbitkan, semua refresh token aktif pasien ini dihapus dari Redis (single-device). Notifikasi WhatsApp dikirim ke `phone_number` pasien via whatsmeow (fire-and-forget; hanya terkirim jika server sudah dipasangkan via `cmd/wa-setup`).

---

### `POST /api/v1/auth/refresh`

**Deskripsi:** Memperbarui access token menggunakan refresh token yang masih valid. Berlaku untuk semua aktor (faskes/nakes/patient). Menerapkan token rotation: refresh token lama dimusnahkan dan diganti yang baru dalam satu operasi atomik.

**Auth:** Public (refresh token di body, bukan header)

**Request**

Body:
```json
{
  "refresh_token": "string — opaque token yang diterima saat login atau refresh sebelumnya"
}
```

**Response — 200 OK**
```json
{
  "message": "token diperbarui",
  "data": {
    "access_token": "string — JWT baru",
    "refresh_token": "string — opaque token baru (token lama tidak bisa dipakai lagi)",
    "expires_in": 43200
  }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | Body tidak valid |
| 401 | Refresh token tidak ditemukan, sudah kadaluarsa, atau terdeteksi reuse |
| 500 | Kegagalan server |

**Catatan:** Jika refresh token yang sudah di-rotate dipakai ulang (reuse detection), seluruh sesi user langsung direvoke dan dikembalikan 401. User harus login ulang dari awal.

---

### `POST /api/v1/auth/logout`

**Deskripsi:** Mencabut refresh token aktif (logout dari perangkat ini). Berlaku untuk semua aktor. Access token yang masih beredar tetap valid sampai expired (≤ 12 jam) — ini trade-off yang disadari dari arsitektur stateless JWT.

**Auth:** Bearer JWT (faskes | nakes | patient)

**Request**

Body:
```json
{
  "refresh_token": "string — refresh token perangkat ini yang ingin di-logout"
}
```

**Response — 200 OK**
```json
{
  "message": "logout berhasil",
  "data": null
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | Body tidak valid |
| 401 | Access token tidak ada / invalid / expired |
| 500 | Kegagalan server |

---

### Faskes & Nakes

---

### `GET /api/v1/nakes/profile`

**Deskripsi:** Mengambil profil lengkap nakes yang sedang login. `nakes_id` otomatis diambil dari JWT — nakes hanya dapat melihat profilnya sendiri, tidak ada path/query param.

**Auth:** Bearer JWT (nakes)

**Request**

_(tidak ada path param, query param, atau body)_

**Response — 200 OK**
```json
{
  "message": "detail profil berhasil diambil",
  "data": {
    "nakes_id": "string — UUID nakes",
    "faskes_id": "string — UUID faskes tempat nakes bertugas",
    "full_name": "string — nama lengkap nakes",
    "role": "string — dokter | kader | admin",
    "nik": "string — 16 digit NIK",
    "alamat": "string — alamat lengkap",
    "phone_number": "string — nomor WA nakes",
    "username": "string — username login nakes",
    "status": "string — active | inactive",
    "enrolled_at": "string — ISO 8601 timestamp",
    "created_at": "string — ISO 8601 timestamp",
    "updated_at": "string — ISO 8601 timestamp"
  }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 401 | Token nakes tidak ada atau tidak valid |
| 404 | Nakes pada JWT tidak ditemukan (mis. akun sudah dihapus) |
| 500 | Kegagalan server |

**Catatan:** `nakes_id` diambil dari JWT — tidak ada parameter dari klien. Shape response identik dengan `GET /api/v1/faskes/nakes/{id}`. `password_hash` tidak pernah dikembalikan.

---

### `GET /api/v1/faskes/profile`

**Deskripsi:** Mengambil profil faskes yang sedang login. `faskes_id` otomatis diambil dari JWT — faskes hanya dapat melihat profilnya sendiri, tidak ada path/query param.

**Auth:** Bearer JWT (faskes)

**Request**

_(tidak ada path param, query param, atau body)_

**Response — 200 OK**
```json
{
  "message": "detail faskes berhasil diambil",
  "data": {
    "faskes_id": "string — UUID faskes",
    "name": "string — nama faskes",
    "type": "string — puskesmas | klinik",
    "address": "string — alamat lengkap",
    "region": "string — wilayah/kota",
    "username": "string — username login faskes",
    "phone_number": "string — nomor WA faskes",
    "status": "string — active | inactive",
    "created_at": "string — ISO 8601 timestamp",
    "updated_at": "string — ISO 8601 timestamp"
  }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 401 | Token faskes tidak ada atau tidak valid |
| 404 | Faskes pada JWT tidak ditemukan (mis. akun sudah dihapus) |
| 500 | Kegagalan server |

**Catatan:** `faskes_id` diambil dari JWT — tidak ada parameter tenant dari klien. `password_hash` tidak pernah dikembalikan. Field `platform_fee_idr` (lihat `docs/erd.md`) sengaja tidak diekspos di endpoint ini.

---

### `GET /api/v1/faskes/nakes`

**Deskripsi:** Mengambil daftar seluruh nakes (dokter/kader/admin) yang terdaftar di faskes yang sedang login. `faskes_id` otomatis diambil dari JWT — faskes hanya dapat melihat nakes miliknya sendiri.

**Auth:** Bearer JWT (faskes)

**Request**

_(tidak ada path param, query param, atau body)_

**Response — 200 OK**
```json
{
  "message": "daftar nakes berhasil diambil",
  "data": [
    {
      "nakes_id": "string — UUID nakes",
      "full_name": "string",
      "role": "string — dokter | kader | admin",
      "username": "string",
      "phone_number": "string",
      "status": "string — active | inactive",
      "enrolled_at": "string — ISO 8601 timestamp"
    }
  ]
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 401 | Token faskes tidak ada atau tidak valid |
| 500 | Kegagalan server |

**Catatan:** Mengembalikan semua nakes (aktif maupun tidak aktif) milik faskes, diurutkan berdasarkan `enrolled_at DESC`. Data diambil berdasarkan `faskes_id` dari JWT — tidak ada parameter tenant dari klien.

---

### `POST /api/v1/faskes/nakes/register/ktp-ocr`

**Deskripsi:** Upload foto KTP calon nakes (dokter/kader/admin). Mengembalikan data ter-ekstrak dari KTP untuk mengisi form registrasi secara otomatis. Tidak menyimpan data ke database — hanya memanggil OCR API eksternal dan mengembalikan hasilnya.

**Auth:** Bearer JWT (faskes)

**Request**

Body: `multipart/form-data`

| Field | Tipe | Keterangan |
|---|---|---|
| `file` | file (wajib) | Gambar KTP JPG/PNG, maks. 5 MB |

**Response — 200 OK**
```json
{
  "message": "KTP berhasil di-scan",
  "data": {
    "nik": "string — 16 digit NIK",
    "full_name": "string — nama lengkap dari KTP",
    "date_of_birth": "string — format YYYY-MM-DD",
    "sex": "string — male | female",
    "alamat": "string — alamat lengkap gabungan dari KTP"
  }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | Field `file` tidak ada, atau format file tidak didukung (bukan JPG/PNG) |
| 401 | Token faskes tidak ada atau tidak valid |
| 422 | KTP tidak terbaca (foto buram, pencahayaan buruk, bukan KTP) |
| 502 | OCR API eksternal tidak tersedia atau API key tidak valid |
| 500 | Kegagalan server |

**Catatan:** `alamat` dikonstruksi dari gabungan field `alamat`, `rt_rw`, `kelurahan`, `kecamatan`, `kota` yang diterima dari OCR API. Biaya Rp 150 per scan berhasil (ditagih oleh provider OCR).

---

### `POST /api/v1/faskes/nakes/register`

**Deskripsi:** Mendaftarkan nakes (dokter/kader/admin) baru ke faskes yang sedang login. `faskes_id` otomatis diambil dari JWT — tidak bisa di-override oleh klien.

**Auth:** Bearer JWT (faskes)

**Request**

Body:
```json
{
  "nik": "string — 16 digit NIK (wajib, harus unik)",
  "full_name": "string — nama lengkap (wajib)",
  "alamat": "string — alamat lengkap (wajib)",
  "phone_number": "string — nomor WA aktif (wajib)",
  "role": "string — dokter | kader | admin (wajib)",
  "username": "string — min 4, max 50 karakter (wajib, harus unik)",
  "password": "string — min 8 karakter (wajib)",
  "specialization": "string | null — spesialisasi dokter (opsional, relevan untuk role dokter)",
  "schedule": [
    { "days": "string — contoh: Senin - Jumat", "time": "string — contoh: 08.00 - 14.00" }
  ]
}
```

**Response — 200 OK**
```json
{
  "message": "nakes berhasil didaftarkan",
  "data": {
    "nakes_id": "string — UUID nakes baru",
    "faskes_id": "string — UUID faskes yang mendaftarkan",
    "full_name": "string",
    "role": "string — dokter | kader | admin",
    "nik": "string",
    "enrolled_at": "string — ISO 8601 timestamp",
    "credentials": {
      "username": "string — username nakes",
      "password": "string — password plaintext (sama dengan input faskes)"
    },
    "wa_warmup": {
      "bot_phone": "string — nomor WA bot; \"\" bila device belum dipasangkan",
      "nakes_link": "string — link wa.me first-contact ke BOT untuk nakes; kosong bila bot belum dipasangkan",
      "nakes_direct_link": "string — link wa.me ke NOMOR NAKES, text=undangan aktivasi (TANPA password) yang memuat nakes_link; faskes klik untuk langsung chat nakes; dihilangkan bila bot belum dipasangkan",
      "status": "string — pending | unavailable"
    }
  }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | Body tidak valid / field wajib kosong / `role` tidak valid |
| 401 | Token faskes tidak ada atau tidak valid |
| 409 | NIK atau username sudah terdaftar |
| 500 | Kegagalan server |

**Catatan:** Nakes langsung berstatus `active` setelah registrasi. Password di-hash dengan bcrypt sebelum disimpan. `specialization` dan `schedule` bersifat opsional — dapat diisi saat registrasi atau diisi belakangan. `hospital` **tidak dikirim dari klien** — server otomatis mengisi kolom ini dari `name` faskes yang melakukan registrasi (diambil dari JWT). `schedule` adalah array objek `{days, time}` yang disimpan sebagai JSONB di kolom `nakes.schedule`; array kosong atau field tidak dikirim → kolom tetap `null`. Kolom-kolom ini dibaca oleh `GET /api/v1/patients/assigned-nakes`.

Pengiriman kredensial memakai **alur warm-up** (sama seperti registrasi pasien — lihat penjelasan lengkap di endpoint registrasi pasien). Backend **tidak** mengirim kredensial via WhatsApp lebih dulu karena WhatsApp memblokir pengiriman ke kontak baru (server error 463 / *reachout time-lock*). Sebagai gantinya, `data.wa_warmup.nakes_link` berisi link `wa.me` ke **bot** yang harus dibuka nakes untuk **menghubungi bot lebih dulu**; setelah nakes mengirim pesan, bot otomatis membalas kredensial. `data.credentials` mengembalikan username + password **sekali** sebagai kanal cadangan terjamin. `status` bernilai `pending` bila bot siap (device sudah dipasangkan via `cmd/wa-setup`) atau `unavailable` bila belum. Percobaan dicatat ke tabel `notifications` (`message_type=credential_blast`, `status=queued`→`sent`) untuk audit; **payload tidak menyimpan password**.

`data.wa_warmup.nakes_direct_link` adalah link `wa.me` yang menunjuk ke **nomor nakes sendiri** (bukan bot), dengan teks undangan aktivasi sudah terisi (`text=`). Faskes cukup **mengklik** link ini → WhatsApp faskes langsung membuka chat ke nakes dengan pesan siap kirim (menggantikan alur salin-tempel manual). Teks berisi sapaan + username + `nakes_link` (link bot), **TANPA password** — password tetap hanya dikirim bot→nakes setelah nakes warm-up via `nakes_link` di dalam pesan. Nomor nakes dinormalkan ke format internasional. Field dihilangkan bila bot belum dipasangkan.

---

### `GET /api/v1/faskes/nakes/{id}`

**Deskripsi:** Mengambil profil lengkap satu nakes (dokter/kader/admin) milik faskes yang sedang login. `faskes_id` otomatis diambil dari JWT — faskes hanya dapat melihat detail nakes miliknya sendiri.

**Auth:** Bearer JWT (faskes)

**Request**

Path params:
| Param | Tipe | Keterangan |
|---|---|---|
| `id` | string (UUID) | ID nakes yang akan dilihat detailnya |

**Response — 200 OK**
```json
{
  "message": "detail nakes berhasil diambil",
  "data": {
    "nakes_id": "string — UUID nakes",
    "faskes_id": "string — UUID faskes",
    "full_name": "string — nama lengkap nakes",
    "role": "string — dokter | kader | admin",
    "nik": "string — 16 digit NIK",
    "alamat": "string — alamat lengkap",
    "phone_number": "string — nomor WA nakes",
    "username": "string — username login nakes",
    "status": "string — active | inactive",
    "enrolled_at": "string — ISO 8601 timestamp",
    "created_at": "string — ISO 8601 timestamp",
    "updated_at": "string — ISO 8601 timestamp"
  }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | `id` kosong |
| 401 | Token faskes tidak ada atau tidak valid |
| 404 | Nakes tidak ditemukan, atau nakes bukan milik faskes yang sedang login |
| 500 | Kegagalan server |

**Catatan:** `faskes_id` diambil dari JWT — faskes hanya bisa melihat nakes miliknya sendiri. Nakes milik faskes lain dikembalikan sebagai 404 (bukan 403) agar keberadaannya tidak bocor lintas tenant (konsisten dengan `GET /api/v1/faskes/patients/{id}` dan `PATCH /api/v1/faskes/nakes/{id}/status`). `password_hash` tidak pernah dikembalikan.

---

### `PATCH /api/v1/faskes/nakes/{id}/status`

**Deskripsi:** Mengubah status keaktifan seorang nakes (dokter/kader/admin) menjadi `active` atau `inactive`. Dipakai faskes untuk menonaktifkan/mengaktifkan kembali akun nakes. Nakes berstatus `inactive` tidak bisa login (lihat error 401 pada `POST /api/v1/nakes/auth/login`).

**Auth:** Bearer JWT (faskes)

**Request**

Path params:
| Param | Tipe | Keterangan |
|---|---|---|
| `id` | string (UUID) | ID nakes yang akan diubah statusnya |

Body:
```json
{
  "status": "string — active | inactive (wajib)"
}
```

**Response — 200 OK**
```json
{
  "message": "status nakes berhasil diperbarui",
  "data": {
    "nakes_id": "string — UUID nakes",
    "full_name": "string",
    "status": "string — active | inactive"
  }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | Body tidak valid / `status` bukan `active` atau `inactive` / `id` kosong |
| 401 | Token faskes tidak ada atau tidak valid |
| 404 | Nakes tidak ditemukan, atau nakes bukan milik faskes yang sedang login |
| 500 | Kegagalan server |

**Catatan:** `faskes_id` diambil dari JWT — faskes hanya bisa mengubah nakes miliknya sendiri. Nakes milik faskes lain dikembalikan sebagai 404 (bukan 403) agar keberadaannya tidak bocor lintas tenant. Operasi idempoten: mengirim status yang sama dengan status saat ini tetap mengembalikan 200.

---

### Patients

---

### `GET /api/v1/faskes/patients`

**Deskripsi:** Mengambil daftar seluruh pasien (semua status: `active` maupun `inactive`) yang dimiliki faskes yang sedang login, dengan pagination. Setiap item menyertakan risk score terbaru pasien dari tabel `risk_scores`. `faskes_id` otomatis diambil dari JWT — faskes hanya dapat melihat pasien miliknya sendiri.

**Auth:** Bearer JWT (faskes)

**Request**

Query params:
| Param | Tipe | Wajib | Default | Keterangan |
|---|---|---|---|---|
| `page` | integer | Tidak | 1 | Nomor halaman (mulai dari 1) |
| `size` | integer | Tidak | 20 | Jumlah item per halaman (maks. 100) |

**Response — 200 OK**
```json
{
  "message": "daftar pasien berhasil diambil",
  "data": [
    {
      "patient_id": "string — UUID pasien",
      "full_name": "string — nama lengkap pasien",
      "nik": "string — 16 digit NIK",
      "sex": "string — male | female",
      "age": 58,
      "disease_type": "string — diabetes_t2 | hypertension | both",
      "phone_number": "string — nomor WA pasien",
      "companion_name": "string — nama pendamping",
      "companion_phone": "string — nomor WA pendamping",
      "status": "string — active | inactive",
      "enrolled_at": "string — ISO 8601 timestamp",
      "health_score": "integer | null — skor risiko 0-100 dari risk_scores.score; null jika pasien belum pernah di-score",
      "risk_status": "string | null — aman | waswas | bahaya; null jika belum di-score",
      "top_factors": "array | null — [{feature, shap_value, direction}] faktor SHAP teratas; null jika belum di-score"
    }
  ],
  "paging": {
    "page": 1,
    "size": 20,
    "total_item": 8,
    "total_page": 1
  }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 401 | Token faskes tidak ada atau tidak valid |
| 500 | Kegagalan server |

**Catatan:** Mengembalikan semua pasien (aktif maupun tidak aktif) milik faskes, diurutkan berdasarkan `enrolled_at DESC`. `age` dihitung dari `date_of_birth` (0 jika tanggal lahir kosong). Data diambil berdasarkan `faskes_id` dari JWT — tidak ada parameter tenant dari klien. Field `health_score`, `risk_status`, dan `top_factors` diambil dari baris `risk_scores` paling baru per pasien (via CTE + `DISTINCT ON`); bernilai `null` jika pasien belum pernah di-score. Berbeda dari `GET /api/v1/nakes/dashboard/patient-queue` yang diurutkan berdasarkan risk score dan hanya menampilkan pasien `active`.

---


### `GET /api/v1/faskes/patients/{id}`

**Deskripsi:** Mengambil profil lengkap satu pasien milik faskes yang sedang login. `faskes_id` otomatis diambil dari JWT — faskes hanya dapat melihat detail pasien miliknya sendiri.

**Auth:** Bearer JWT (faskes)

**Request**

Path params:
| Param | Tipe | Keterangan |
|---|---|---|
| `id` | string (UUID) | ID pasien yang akan dilihat detailnya |

**Response — 200 OK**
```json
{
  "message": "detail pasien berhasil diambil",
  "data": {
    "patient_id": "string — UUID pasien",
    "faskes_id": "string — UUID faskes",
    "assigned_nakes_id": "string — UUID nakes penanggung jawab",
    "assigned_nakes_name": "string — nama nakes penanggung jawab, kosong jika nakes tidak ditemukan",
    "full_name": "string — nama lengkap pasien",
    "nik": "string — 16 digit NIK",
    "date_of_birth": "string — YYYY-MM-DD, kosong jika tidak ada",
    "sex": "string — male | female",
    "age": 58,
    "alamat": "string — alamat lengkap",
    "phone_number": "string — nomor WA pasien",
    "companion_name": "string — nama pendamping",
    "companion_phone": "string — nomor WA pendamping",
    "disease_type": "string — diabetes_t2 | hypertension | both",
    "username": "string — username login pasien",
    "status": "string — active | inactive",
    "enrolled_at": "string — ISO 8601 timestamp",
    "created_at": "string — ISO 8601 timestamp",
    "updated_at": "string — ISO 8601 timestamp"
  }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | `id` kosong |
| 401 | Token faskes tidak ada atau tidak valid |
| 404 | Pasien tidak ditemukan, atau pasien bukan milik faskes yang sedang login |
| 500 | Kegagalan server |

**Catatan:** `faskes_id` diambil dari JWT — faskes hanya bisa melihat pasien miliknya sendiri. Pasien milik faskes lain dikembalikan sebagai 404 (bukan 403) agar keberadaannya tidak bocor lintas tenant (konsisten dengan `PATCH /api/v1/faskes/nakes/{id}/status`). `age` dihitung dari `date_of_birth` (0 jika kosong). `password_hash` tidak pernah dikembalikan. `assigned_nakes_name` di-resolve dari `assigned_nakes_id`; bila nakes tersebut tidak ditemukan, field dibiarkan kosong dan detail pasien tetap dikembalikan.

---

### `POST /api/v1/faskes/patients/register/ktp-ocr`

**Deskripsi:** Upload foto KTP calon pasien. Mengembalikan data ter-ekstrak dari KTP untuk mengisi form registrasi pasien secara otomatis. Tidak menyimpan data ke database.

**Auth:** Bearer JWT (faskes)

**Request**

Body: `multipart/form-data`

| Field | Tipe | Keterangan |
|---|---|---|
| `file` | file (wajib) | Gambar KTP JPG/PNG, maks. 5 MB |

**Response — 200 OK**
```json
{
  "message": "KTP berhasil di-scan",
  "data": {
    "nik": "string — 16 digit NIK",
    "full_name": "string — nama lengkap dari KTP",
    "date_of_birth": "string — format YYYY-MM-DD",
    "sex": "string — male | female",
    "alamat": "string — alamat lengkap gabungan dari KTP"
  }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | Field `file` tidak ada, atau format file tidak didukung |
| 401 | Token faskes tidak ada atau tidak valid |
| 422 | KTP tidak terbaca |
| 502 | OCR API eksternal tidak tersedia atau API key tidak valid |
| 500 | Kegagalan server |

---

### `POST /api/v1/faskes/patients/register`

**Deskripsi:** Mendaftarkan pasien baru ke faskes yang sedang login. `faskes_id` otomatis diambil dari JWT faskes — tidak bisa di-override oleh klien. `assigned_nakes_id` (dokter penanggung jawab) dikirim faskes di body dan harus merujuk ke nakes milik faskes ini.

**Auth:** Bearer JWT (faskes)

**Request**

Body:
```json
{
  "assigned_nakes_id": "string — UUID nakes penanggung jawab (wajib, harus milik faskes ini)",
  "nik": "string — 16 digit NIK (wajib, harus unik)",
  "full_name": "string — nama lengkap (wajib)",
  "date_of_birth": "string — format YYYY-MM-DD (wajib)",
  "sex": "string — male | female (wajib)",
  "alamat": "string — alamat lengkap (wajib)",
  "phone_number": "string — nomor WA aktif pasien (wajib)",
  "companion_name": "string — nama pendamping/keluarga (wajib)",
  "companion_phone": "string — nomor WA pendamping (wajib)",
  "disease_type": "string — diabetes_t2 | hypertension | both (wajib)",
  "username": "string — min 4, max 50 karakter (wajib, harus unik)",
  "password": "string — min 8 karakter (wajib)"
}
```

**Response — 200 OK**
```json
{
  "message": "pasien berhasil didaftarkan",
  "data": {
    "patient_id": "string — UUID pasien baru",
    "faskes_id": "string — UUID faskes",
    "full_name": "string",
    "nik": "string",
    "disease_type": "string — diabetes_t2 | hypertension | both",
    "enrolled_at": "string — ISO 8601 timestamp",
    "credentials": {
      "username": "string — username pasien",
      "password": "string — password plaintext (sama dengan input faskes)"
    },
    "wa_warmup": {
      "bot_phone": "string — nomor WA bot; \"\" bila device belum dipasangkan",
      "patient_link": "string — link wa.me first-contact ke BOT untuk pasien; kosong bila bot belum dipasangkan",
      "companion_link": "string — link wa.me ke BOT untuk pendamping; dihilangkan bila companion_phone kosong",
      "patient_direct_link": "string — link wa.me ke NOMOR PASIEN, text=undangan aktivasi (username + patient_link, TANPA password); faskes klik untuk langsung chat pasien; dihilangkan bila bot belum dipasangkan",
      "companion_direct_link": "string — link wa.me ke NOMOR PENDAMPING, text=undangan aktivasi yang memuat companion_link; dihilangkan bila bot belum dipasangkan atau companion_phone kosong",
      "status": "string — pending | unavailable"
    }
  }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | Body tidak valid / field wajib kosong / `sex` atau `disease_type` tidak valid / `date_of_birth` bukan YYYY-MM-DD / `assigned_nakes_id` tidak valid atau bukan milik faskes ini |
| 401 | Token faskes tidak ada atau tidak valid |
| 409 | NIK atau username sudah terdaftar |
| 500 | Kegagalan server |

**Catatan:** Pasien langsung berstatus `active`. `assigned_nakes_id` di-set dari body request dan divalidasi harus merujuk ke nakes milik faskes yang sedang login (jika tidak ada atau milik faskes lain → 400, pesan diseragamkan agar tidak bocor lintas tenant). Password di-hash dengan bcrypt sebelum disimpan.

**Pengiriman kredensial memakai alur _warm-up_.** WhatsApp memblokir pengiriman pesan ke **kontak baru** yang belum pernah menghubungi bot lebih dulu (server membalas error `463` / `NackCallerReachoutTimelocked` — pembatasan anti-spam tingkat akun, bukan bug). Karena itu backend **tidak** mengirim kredensial duluan. Sebagai gantinya:

1. `data.credentials` mengembalikan username + password (plaintext, sama dengan input faskes) **sekali** sebagai **kanal cadangan terjamin** — faskes selalu bisa menyampaikan login langsung ke pasien/pendamping.
2. `data.wa_warmup.patient_link` dan `data.wa_warmup.companion_link` berisi link `wa.me` ke **bot** yang membuat penerima **menghubungi bot lebih dulu** (faskes menampilkan/meneruskan link ini). Begitu pasien/pendamping mengirim pesan ke bot, percakapan menjadi "hangat" dan **bot otomatis membalas kredensial** — tidak lagi dianggap kontak dingin, sehingga tidak kena error 463.
   - `data.wa_warmup.patient_direct_link` dan `companion_direct_link` adalah link `wa.me` yang menunjuk ke **nomor pasien/pendamping sendiri** (bukan bot), dengan teks undangan aktivasi sudah terisi (`text=`). Karena pasien biasanya **tidak hadir** saat didaftarkan, faskes cukup **mengklik** link ini → WhatsApp faskes langsung membuka chat ke pasien/pendamping dengan pesan siap kirim (menggantikan alur salin-tempel manual). Teks berisi sapaan + (untuk pasien) username + link bot (`patient_link`/`companion_link`), **TANPA password** — password tetap hanya dikirim bot setelah penerima warm-up via link bot di dalam pesan. Nomor penerima dinormalkan ke format internasional. Tiap field dihilangkan bila bot belum dipasangkan atau penerima tidak ada (mis. tanpa pendamping).
3. `data.wa_warmup.status` bernilai `pending` bila bot siap (device sudah dipasangkan via `cmd/wa-setup`), atau `unavailable` bila belum (semua link kosong — faskes harus menyampaikan kredensial manual).

Kredensial menunggu disimpan sementara di Redis (key `pending_credential:{phone}`, TTL 72 jam, **tanpa** disimpan permanen di Postgres) sampai penerima menghubungi bot. Setiap pendaftaran mencatat baris audit ke tabel `notifications` (`message_type=credential_blast`, `status=queued` → `sent` setelah terkirim) — **payload audit tidak menyimpan password**. Kegagalan WA/Redis **tidak** menggagalkan registrasi: pasien tetap tersimpan dan response tetap 200.

---

### `GET /api/v1/faskes/patients/{id}/baseline`

**Deskripsi:** Mengambil **baseline klinis terbaru** (lengkap, 33 fitur) milik satu pasien. Dipakai faskes untuk pre-fill form sebelum mencatat baseline versi baru. `faskes_id` dari JWT — hanya pasien milik faskes ini yang dapat diakses.

**Auth:** Bearer JWT (faskes)

**Request**

Path params:
| Param | Tipe | Keterangan |
|---|---|---|
| `id` | string (UUID) | ID pasien |

**Response — 200 OK**
```json
{
  "message": "baseline terbaru berhasil diambil",
  "data": {
    "id": "string — UUID baris baseline",
    "patient_id": "string — UUID pasien",
    "recorded_at": "string — ISO 8601 timestamp pencatatan",
    "recorded_by_nakes_id": "string | null — UUID nakes pencatat (null untuk baseline registrasi awal)",
    "recorded_by_nakes_name": "string — nama nakes pencatat, kosong bila tidak ada",
    "notes": "string | null — catatan/alasan",
    "age_years": "int", "sex": "string — male | female",
    "bmi": "number", "bmi_category": "string — underweight | normal | overweight | obese",
    "waist_circumference_cm": "number", "central_obesity": "bool",
    "smoking_status": "string — never | former | current", "alcohol_use": "bool",
    "physical_activity": "string — sedentary | light | moderate | active",
    "family_history_diabetes": "bool", "family_history_cvd": "bool",
    "systolic_bp_mmhg": "int", "diastolic_bp_mmhg": "int", "hypertension_status": "string",
    "fasting_glucose_mgdl": "number", "hba1c_pct": "number", "diabetes_status": "string",
    "total_cholesterol_mgdl": "number", "hdl_mgdl": "number", "ldl_mgdl": "number", "triglycerides_mgdl": "number",
    "cvd_risk_10yr_pct": "number", "cvd_risk_category": "string — low | moderate | high | very_high",
    "on_antihypertensive": "bool", "on_antidiabetic": "bool", "on_statin": "bool",
    "target_risk": "string", "egfr": "number", "uacr": "number",
    "cluster_id": "int | null", "diagnosis_cluster": "string | null", "clinical_group": "string | null"
  }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | `id` kosong |
| 401 | Token faskes tidak ada atau tidak valid |
| 404 | Pasien tidak ditemukan / bukan milik faskes ini, **atau** pasien belum punya baseline sama sekali |
| 500 | Kegagalan server |

**Catatan:** Mengembalikan baris baseline dengan `recorded_at` paling baru. Pasien milik faskes lain dikembalikan sebagai 404 (bukan 403) agar tidak bocor lintas tenant.

---

### `POST /api/v1/faskes/patients/{id}/baseline`

**Deskripsi:** Mencatat **versi baseline klinis baru** untuk pasien (mis. hasil kontrol bulanan Prolanis). Bersifat **insert-only / log**: baris baru ditambahkan, baseline lama **tidak** ditimpa, sehingga progres baseline pasien dapat ditelusuri dari waktu ke waktu.

**Auth:** Bearer JWT (faskes)

**Request**

Path params:
| Param | Tipe | Keterangan |
|---|---|---|
| `id` | string (UUID) | ID pasien |

Body:
```json
{
  "recorded_by_nakes_id": "string — UUID nakes pencatat (wajib, harus milik faskes ini)",
  "recorded_at": "string — YYYY-MM-DD (opsional; default waktu sekarang)",
  "notes": "string — catatan/alasan (opsional)",
  "baseline": {
    "age_years": "int (wajib, 0–150)",
    "sex": "string — male | female (wajib)",
    "bmi": "number (wajib, 5–100)",
    "bmi_category": "string — underweight | normal | overweight | obese (wajib)",
    "waist_circumference_cm": "number (wajib, 20–250)",
    "central_obesity": "bool (wajib)",
    "smoking_status": "string — never | former | current (wajib)",
    "alcohol_use": "bool (wajib)",
    "physical_activity": "string — sedentary | light | moderate | active (wajib)",
    "family_history_diabetes": "bool (wajib)",
    "family_history_cvd": "bool (wajib)",
    "systolic_bp_mmhg": "int (wajib, 40–300)",
    "diastolic_bp_mmhg": "int (wajib, 20–200)",
    "hypertension_status": "string (wajib)",
    "fasting_glucose_mgdl": "number (wajib, 20–1000)",
    "hba1c_pct": "number (wajib, 1–20)",
    "diabetes_status": "string (wajib)",
    "total_cholesterol_mgdl": "number (wajib, 50–1000)",
    "hdl_mgdl": "number (wajib, 5–200)",
    "ldl_mgdl": "number (wajib, 5–600)",
    "triglycerides_mgdl": "number (wajib, 10–5000)",
    "cvd_risk_10yr_pct": "number (0–100)",
    "cvd_risk_category": "string — low | moderate | high | very_high (wajib)",
    "on_antihypertensive": "bool (wajib)",
    "on_antidiabetic": "bool (wajib)",
    "on_statin": "bool (wajib)",
    "target_risk": "string (wajib)",
    "egfr": "number (wajib, 0–200)",
    "uacr": "number (>= 0)",
    "cluster_id": "int | null (opsional)",
    "diagnosis_cluster": "string | null (opsional)",
    "clinical_group": "string | null (opsional)"
  }
}
```

**Response — 201 Created**

Sama dengan body data pada `GET /api/v1/faskes/patients/{id}/baseline` (baris baseline yang baru dibuat, termasuk `id`, `recorded_at`, `recorded_by_nakes_id`, `recorded_by_nakes_name`, `notes`).

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | Body tidak valid / field baseline wajib kosong / enum tidak valid / `recorded_at` bukan YYYY-MM-DD / `recorded_by_nakes_id` tidak valid atau bukan milik faskes ini |
| 401 | Token faskes tidak ada atau tidak valid |
| 404 | Pasien tidak ditemukan / bukan milik faskes ini |
| 500 | Kegagalan server |

**Catatan:** **Insert-only** — tiap pemanggilan menambah satu baris baseline baru (tidak ada UPDATE/DELETE). `payload baseline` sama persis dengan objek `baseline` saat registrasi pasien. `recorded_by_nakes_id` divalidasi harus milik faskes ini (jika tidak → 400, pesan diseragamkan agar tidak bocor lintas tenant). **Efek ke skoring ML:** baseline adalah setengah payload model; setelah baris baru tercatat, skoring harian berikutnya otomatis memakai baseline **terbaru** ini.

---

### `GET /api/v1/faskes/patients/{id}/baseline/history`

**Deskripsi:** Menampilkan **progress baseline** pasien (metrik kunci, paginated) **beserta tren health score** pasien sebagai deret terpisah, dari waktu ke waktu, terbaru-dulu, untuk grafik/tren di dashboard faskes. `faskes_id` dari JWT.

**Auth:** Bearer JWT (faskes)

**Request**

Path params:
| Param | Tipe | Keterangan |
|---|---|---|
| `id` | string (UUID) | ID pasien |

Query params:
| Param | Tipe | Wajib | Keterangan |
|---|---|---|---|
| `page` | int | tidak | Default 1 (untuk `baseline_history`) |
| `size` | int | tidak | Default 20, maks 100 (untuk `baseline_history`) |
| `score_limit` | int | tidak | Jumlah titik tren health score, default 90, maks 365 |

**Response — 200 OK**
```json
{
  "message": "riwayat baseline berhasil diambil",
  "data": {
    "baseline_history": [
      {
        "id": "string — UUID baris baseline",
        "recorded_at": "string — ISO 8601 timestamp",
        "recorded_by_nakes_name": "string — nama nakes pencatat, kosong bila tidak ada",
        "notes": "string | null",
        "bmi": "number", "bmi_category": "string",
        "systolic_bp_mmhg": "int", "diastolic_bp_mmhg": "int", "hypertension_status": "string",
        "fasting_glucose_mgdl": "number", "hba1c_pct": "number", "diabetes_status": "string",
        "total_cholesterol_mgdl": "number", "hdl_mgdl": "number", "ldl_mgdl": "number", "triglycerides_mgdl": "number",
        "cvd_risk_10yr_pct": "number", "cvd_risk_category": "string",
        "egfr": "number", "uacr": "number"
      }
    ],
    "health_score_history": [
      {
        "score": "int — 0–100",
        "status": "string — aman | waswas | bahaya",
        "scored_at": "string — ISO 8601 timestamp"
      }
    ]
  },
  "paging": { "page": 1, "size": 20, "total_item": 3, "total_page": 1 }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | `id` kosong |
| 401 | Token faskes tidak ada atau tidak valid |
| 404 | Pasien tidak ditemukan / bukan milik faskes ini |
| 500 | Kegagalan server |

**Catatan:** `data` adalah objek dengan dua deret terpisah. `baseline_history` memuat **metrik kunci** (subset baseline lengkap) dan **dipaginasi** oleh `page`/`size` (`paging` mengacu ke deret ini), diurutkan `recorded_at` menurun. `health_score_history` adalah tren health score harian dari `risk_scores` (diurutkan `scored_at` menurun, dibatasi `score_limit`) — sengaja **deret terpisah** karena health score di-update jauh lebih sering daripada baseline. Pasien tanpa baseline → `baseline_history: []`; pasien yang belum pernah di-score → `health_score_history: []`.

---

### Dashboard Nakes

---

### `GET /api/v1/nakes/dashboard/summary`

**Deskripsi:** Mengambil ringkasan statistik dashboard untuk faskes yang sedang login — total pasien aktif, jumlah pasien dengan risiko bahaya, dan jumlah pasien dengan status aman. Data didasarkan pada risk score terbaru per pasien.

**Auth:** Bearer JWT (nakes)

**Request**

_(tidak ada path param, query param, atau body)_

**Response — 200 OK**
```json
{
  "message": "ringkasan dashboard berhasil diambil",
  "data": {
    "total_pasien": 8,
    "risiko_bahaya": 2,
    "status_aman": 3
  }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 401 | Token nakes tidak ada atau tidak valid |
| 500 | Kegagalan server |

**Catatan:** `faskes_id` diambil dari JWT — nakes hanya dapat melihat data faskes miliknya. Pasien yang belum memiliki risk score tetap dihitung di `total_pasien` namun tidak masuk ke `risiko_bahaya` maupun `status_aman`.

---

### `GET /api/v1/nakes/dashboard/patient-queue`

**Deskripsi:** Mengambil daftar pasien aktif dari faskes yang sedang login, diurutkan berdasarkan prioritas risiko/triase (AI Auto-Sorted): status terburuk dulu (`bahaya` → `waswas` → `aman`), lalu `health_score` TERENDAH dulu di tiap status; pasien yang belum punya skor ditaruh paling bawah. Mendukung pagination.

**Auth:** Bearer JWT (nakes)

**Request**

Query params:
| Param | Tipe | Wajib | Default | Keterangan |
|---|---|---|---|---|
| `page` | integer | Tidak | 1 | Nomor halaman (mulai dari 1) |
| `size` | integer | Tidak | 20 | Jumlah item per halaman (maks. 100) |

**Response — 200 OK**
```json
{
  "message": "antrian prioritas pasien berhasil diambil",
  "data": [
    {
      "patient_id": "string — UUID pasien",
      "full_name": "string — nama lengkap pasien",
      "age": 58,
      "disease_type": "string — diabetes_t2 | hypertension | both",
      "risk_score": 92,
      "risk_label": "string — kritis | sedang | rendah",
      "status": "string — bahaya | waswas | aman",
      "main_factor": "string — label Indonesia faktor SHAP tertinggi, kosong jika belum ada skor"
    }
  ],
  "paging": {
    "page": 1,
    "size": 20,
    "total_item": 8,
    "total_page": 1
  }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 401 | Token nakes tidak ada atau tidak valid |
| 500 | Kegagalan server |

**Catatan:**
- `risk_score` adalah `health_score` (0–100, **TINGGI = sehat**). `risk_label` diturunkan terbalik dari skor: `≤ 40` → `kritis`, `41–70` → `sedang`, `> 70` → `rendah`.
- `status` adalah nilai enum dari kolom `risk_scores.status` (`bahaya | waswas | aman`).
- `main_factor` diambil dari elemen pertama array `top_factors` (SHAP value tertinggi) di `risk_scores`, lalu diterjemahkan ke label Indonesia.
- Pasien tanpa risk score tetap muncul di antrian dengan `risk_score: 0`, `risk_label: "rendah"`, `status: "aman"`, `main_factor: ""`.
- `faskes_id` diambil dari JWT — tidak bisa di-override oleh klien.

---

### `GET /api/v1/nakes/patients/:id`

**Deskripsi:** Mengambil detail lengkap satu pasien dari sudut pandang nakes (hanya untuk pasien dalam satu faskes), mencakup profil, baseline klinis, log harian terbaru, dan faktor risiko AI.

**Auth:** Bearer JWT (nakes)

**Request**

Path params:
| Param | Tipe | Keterangan |
|---|---|---|
| `id` | string (UUID) | ID pasien |

**Response — 200 OK**
```json
{
  "message": "detail pasien untuk nakes berhasil diambil",
  "data": {
    "patient_detail": {
      "patient_id": "string — UUID",
      "full_name": "string",
      "disease_type": "diabetes_t2 | hypertension | both",
      "...": "field pasien lainnya"
    },
    "baseline": {
      "id": "string",
      "recorded_at": "2026-06-30T10:00:00Z",
      "age_years": 58,
      "bmi": 29.4,
      "hba1c_pct": 10.2,
      "...": "field baseline lainnya"
    },
    "daily_logs": [
      {
        "date": "2026-06-30",
        "blood_sugar": 215,
        "weight": 70,
        "systolic": 140,
        "diastolic": 90,
        "health_score": 72
      }
    ],
    "risk": {
      "score": 72,
      "status": "waswas",
      "scoring_mode": "cohort",
      "top_factors": [
        {
          "feature": "hba1c",
          "shap_value": 2.4,
          "direction": "positive"
        }
      ]
    },
    "health_score_history": [
      {
        "score": 72,
        "status": "waswas",
        "scored_at": "2026-06-30T10:00:00Z"
      }
    ]
  }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 401 | Token nakes tidak ada atau tidak valid |
| 404 | Pasien tidak ditemukan atau bukan bagian dari faskes nakes |
| 500 | Kegagalan server |

**Catatan:** `baseline`, `daily_logs`, dan `risk` bisa bernilai null atau kosong jika pasien belum memiliki data. `faskes_id` divalidasi dari JWT.

---

### `GET /api/v1/nakes/patients/:id/summary`

**Deskripsi:** Ringkasan kesehatan klinis satu pasien pada window 7/14/30 hari — gabungan angka agregat (dihitung backend dari `health_logs`) + narasi (AI/Gemini) bernada klinis untuk nakes. Window hanya tersedia bila riwayat data pasien menutupinya.

**Auth:** Bearer JWT (nakes)

**Request**

Path params:
| Param | Tipe | Keterangan |
|---|---|---|
| `id` | string (UUID) | ID pasien (harus milik faskes nakes yang login) |

Query params:
| Param | Tipe | Wajib | Default | Keterangan |
|---|---|---|---|---|
| `window` | integer | Tidak | 7 | Salah satu dari `7`, `14`, `30` (hari) |

**Response — 200 OK (data cukup)**
```json
{
  "message": "ringkasan kesehatan pasien berhasil diambil",
  "data": {
    "window": 7,
    "available": true,
    "available_windows": [7, 14, 30],
    "period": { "start": "2026-06-24", "end": "2026-06-30" },
    "coverage": { "logged_days": 5, "window_days": 7, "streak_days": 3 },
    "aggregates": {
      "glucose": { "avg_mgdl": 142.5, "min_mgdl": 98, "max_mgdl": 210, "count": 6 },
      "blood_pressure": { "avg_systolic": 134.2, "avg_diastolic": 85.1, "count": 5 },
      "med_adherence": { "adherence_rate_pct": 80, "count": 5 },
      "nutrition": { "avg_kcal_per_day": 1850.4, "avg_carbs_g_per_day": 210.3, "avg_sodium_mg_per_day": 1200, "meal_count": 8 },
      "activity": { "avg_minutes_per_day": 25, "total_minutes": 75, "count": 3 },
      "sleep": { "avg_hours": 6.5, "count": 4 },
      "stress": { "avg_level": 4.2, "count": 4 },
      "weight": { "start_kg": 70.5, "latest_kg": 70.1, "delta_kg": -0.4, "count": 2 }
    },
    "risk": { "score": 72, "status": "waswas", "scored_at": "2026-06-30T01:00:00Z" },
    "narrative": "string — ringkasan klinis 3-5 kalimat dari Gemini",
    "generated_at": "2026-06-30T08:40:00+07:00"
  }
}
```

**Response — 200 OK (data belum cukup untuk window diminta)**
```json
{
  "message": "ringkasan kesehatan pasien berhasil diambil",
  "data": {
    "window": 30,
    "available": false,
    "available_windows": [7],
    "history_days": 9,
    "message": "Data pasien baru mencakup 9 hari, sedangkan ringkasan 30 hari membutuhkan minimal 30 hari pencatatan. Terus catat kondisi harian agar ringkasan ini tersedia.",
    "narrative": "",
    "generated_at": "2026-06-30T08:40:00+07:00"
  }
}
```
> `history_days` = jumlah hari riwayat pencatatan pasien (log pertama s.d. hari ini, WIB); `0` bila pasien belum pernah mengisi. `message` berisi penjelasan ramah; kosong saat `available:true`. `history_days` juga selalu ada pada response `available:true`.

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | `window` bukan 7/14/30 |
| 401 | Token nakes tidak ada atau tidak valid |
| 404 | Pasien tidak ditemukan **atau** milik faskes lain (tenant isolation — tidak dibedakan) |
| 500 | Kegagalan server |

**Catatan:**
- Tiap sub-objek `aggregates` bernilai `null` bila tidak ada data metrik tsb di window; bagian `period`/`coverage`/`aggregates` dihilangkan saat `available:false`.
- `available_windows` berisi window yang ditopang riwayat: sebuah window `w` valid bila rentang hari dari log pertama s.d. hari ini ≥ `w`. Frontend memakai ini untuk menampilkan pilihan window yang valid.
- Hari di-bucket pada zona **Asia/Jakarta (WIB)**.
- **Efek samping:** hasil di-cache di Redis dengan key `summary:nakes:{patientId}:{window}:{YYYY-MM-DD WIB}` (TTL ~24 jam) dan memanggil Gemini API saat cache miss. Jika Gemini gagal, response tetap 200 dengan angka agregat dan `narrative` fallback (tidak di-cache).
- `faskes_id` diambil dari JWT — pasien lintas tenant dikembalikan 404.

---

### `GET /api/v1/nakes/consultations`

**Deskripsi:** Mengambil daftar seluruh konsultasi dari pasien yang ditugaskan ke nakes yang sedang login, diurutkan dari yang terbaru. `nakes_id` dan `faskes_id` diambil dari JWT.

**Auth:** Bearer JWT (nakes)

**Request**

_(tidak ada path param, query param, atau body)_

**Response — 200 OK**
```json
{
  "message": "daftar konsultasi pasien berhasil diambil",
  "data": [
    {
      "id":               "string — UUID konsultasi",
      "patient_id":       "string — UUID pasien",
      "patient_name":     "string — nama lengkap pasien",
      "complaint_since":  "string — kapan keluhan mulai",
      "complaint_type":   "string — jenis keluhan",
      "complaint_detail": "string — detail keluhan",
      "status":           "string — open | replied",
      "nakes_note":       "string | null — balasan nakes, null jika belum dibalas",
      "replied_at":       "string | null — ISO 8601 timestamp balasan",
      "created_at":       "string — ISO 8601 timestamp"
    }
  ]
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 401 | Token nakes tidak ada atau tidak valid |
| 500 | Kegagalan server |

**Catatan:** Hanya menampilkan konsultasi dari pasien yang `assigned_nakes_id`-nya sama dengan nakes yang sedang login, dan `faskes_id` pasien sesuai JWT (tenant isolation). Konsultasi dari pasien faskes lain tidak pernah muncul.

---

### `POST /api/v1/nakes/consultations/{id}/reply`

**Deskripsi:** Nakes membalas keluhan pasien dengan catatan. Mengubah status konsultasi dari `open` ke `replied` dan membuat notifikasi inbox in-app ke pasien. `nakes_id` diambil dari JWT.

**Auth:** Bearer JWT (nakes)

**Request**

Path params:
| Param | Tipe | Keterangan |
|---|---|---|
| `id` | string (UUID) | ID konsultasi yang akan dibalas |

Body:
```json
{
  "nakes_note": "string — catatan/balasan dokter (wajib, 1–2000 karakter)"
}
```

**Response — 200 OK**
```json
{
  "message": "balasan berhasil dikirim",
  "data": null
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | Body tidak valid / `nakes_note` kosong / `id` tidak diisi |
| 401 | Token nakes tidak ada atau tidak valid |
| 404 | Konsultasi tidak ditemukan |
| 409 | Konsultasi sudah pernah dibalas sebelumnya |
| 500 | Kegagalan server |

**Catatan:** Setelah reply berhasil disimpan ke DB, satu baris disimpan ke tabel **`patient_notifications`** (inbox in-app) dengan `type=consultation_reply`, `title="Balasan dari dokter"`, `body=` isi balasan, dan `read_at=NULL` (belum dibaca) — inilah notifikasi yang dibaca mobile app via `GET /api/v1/patients/notifications`. (Sebelumnya baris ini ditulis ke `notifications` dengan `channel=in_app`; sejak refactor notifikasi, `notifications` murni log transport WA/SMS.) Kegagalan menyimpan notifikasi hanya di-log dan tidak memblok response. Endpoint tidak memvalidasi apakah pasien memang assigned ke nakes ini — hanya nakes dengan JWT valid yang bisa memanggil endpoint ini; validasi kepemilikan konsultasi dapat ditambahkan di fase berikutnya jika dibutuhkan.

---

### Eskalasi (Nakes)

---

### `GET /api/v1/faskes/escalations`

**Deskripsi:** Versi faskes dari antrian eskalasi — identik dengan `GET /api/v1/nakes/escalations` (sama shape, sama logic, sama tenant isolation via `faskes_id` dari JWT) tetapi menerima token faskes. Dipakai oleh portal admin faskes yang login sebagai entitas faskes, bukan sebagai nakes individual.

**Auth:** Bearer JWT (faskes)

**Request** — query params (semua opsional):
| Param | Tipe | Keterangan |
|---|---|---|
| `status` | string | `sent` \| `viewed` \| `acted` \| `dismissed` |
| `tier` | string | `acute_today` \| `trend_this_week` |
| `page` | int | default 1 |
| `size` | int | default 20, maks 100 |

**Response — 200 OK**
```json
{
  "message": "antrian eskalasi berhasil diambil",
  "data": [
    {
      "id": "string — UUID eskalasi",
      "patient_id": "string — UUID pasien",
      "patient_name": "string — nama lengkap pasien",
      "tier": "string — acute_today | trend_this_week",
      "status": "string — sent | viewed | acted | dismissed",
      "risk_score": "integer — skor risk_scores pemicu (0 jika tak ada)",
      "risk_status": "string — aman | waswas | bahaya ('' jika tak ada)",
      "sent_at": "string — ISO 8601",
      "viewed_at": "string | null — ISO 8601",
      "acted_at": "string | null — ISO 8601",
      "created_at": "string — ISO 8601"
    }
  ],
  "paging": { "page": 1, "size": 20, "total_item": 1, "total_page": 1 }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 401 | Token faskes tidak ada / invalid |
| 500 | Kegagalan server |

---

### `PATCH /api/v1/faskes/escalations/{id}/view`

**Deskripsi:** Versi faskes dari mark-viewed — identik dengan `PATCH /api/v1/nakes/escalations/{id}/view` tetapi menerima token faskes.

**Auth:** Bearer JWT (faskes)

**Request** — Path params:
| Param | Tipe | Keterangan |
|---|---|---|
| `id` | string (UUID) | ID eskalasi |

_(tanpa body)_

**Response — 200 OK**
```json
{ "message": "status eskalasi berhasil diperbarui", "data": null }
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | `id` kosong |
| 401 | Token faskes tidak ada / invalid |
| 404 | Eskalasi tidak ditemukan / milik faskes lain |
| 500 | Kegagalan server |

---

### `PATCH /api/v1/faskes/escalations/{id}/act`

**Deskripsi:** Versi faskes dari mark-acted — identik dengan `PATCH /api/v1/nakes/escalations/{id}/act` tetapi menerima token faskes.

**Auth:** Bearer JWT (faskes)

**Request** — Path params:
| Param | Tipe | Keterangan |
|---|---|---|
| `id` | string (UUID) | ID eskalasi |

_(tanpa body)_

**Response — 200 OK**
```json
{ "message": "status eskalasi berhasil diperbarui", "data": null }
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | `id` kosong |
| 401 | Token faskes tidak ada / invalid |
| 404 | Eskalasi tidak ditemukan / milik faskes lain |
| 409 | Eskalasi sudah ditutup (`acted`/`dismissed`) |
| 500 | Kegagalan server |

---

### `PATCH /api/v1/faskes/escalations/{id}/dismiss`

**Deskripsi:** Versi faskes dari mark-dismissed — identik dengan `PATCH /api/v1/nakes/escalations/{id}/dismiss` tetapi menerima token faskes.

**Auth:** Bearer JWT (faskes)

**Request** — Path params:
| Param | Tipe | Keterangan |
|---|---|---|
| `id` | string (UUID) | ID eskalasi |

_(tanpa body)_

**Response — 200 OK**
```json
{ "message": "status eskalasi berhasil diperbarui", "data": null }
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | `id` kosong |
| 401 | Token faskes tidak ada / invalid |
| 404 | Eskalasi tidak ditemukan / milik faskes lain |
| 409 | Eskalasi sudah ditutup (`acted`/`dismissed`) |
| 500 | Kegagalan server |

**Catatan:** Endpoint `feedback` (`PATCH /api/v1/nakes/escalations/{id}/feedback`) tidak ada padanannya di grup faskes karena feedback membutuhkan `nakes_id` dari JWT — faskes token tidak membawa field ini.

---

### `GET /api/v1/nakes/escalations`

**Deskripsi:** Antrean eskalasi klinis untuk faskes yang sedang login, `acute_today` lebih dulu lalu terbaru. `faskes_id` diambil dari JWT (tenant isolation). Mendukung pagination dan filter opsional. Eskalasi `acute_today` dibuat otomatis saat scoring (`POST /api/v1/patients/records`) bila status pasien **bertransisi ke `bahaya`** ATAU ada **pembacaan ekstrem hari ini** (gula `≥ ACUTE_GLUCOSE_HIGH`/`≤ ACUTE_GLUCOSE_LOW`, tensi `≥ ACUTE_SYSTOLIC_HIGH`/`ACUTE_DIASTOLIC_HIGH`). Dilindungi dedup + cooldown (`ACUTE_COOLDOWN_HOURS`, default 24 jam) agar tidak banjir. Eskalasi `trend_this_week` dibuat worker harian saat status `waswas` bertahan beberapa hari.

**Auth:** Bearer JWT (nakes)

**Request** — query params (semua opsional):
| Param | Tipe | Keterangan |
|---|---|---|
| `status` | string | `sent` \| `viewed` \| `acted` \| `dismissed` |
| `tier` | string | `acute_today` \| `trend_this_week` |
| `page` | int | default 1 |
| `size` | int | default 20, maks 100 |

**Response — 200 OK**
```json
{
  "message": "antrian eskalasi berhasil diambil",
  "data": [
    {
      "id": "string — UUID eskalasi",
      "patient_id": "string — UUID pasien",
      "patient_name": "string — nama lengkap pasien",
      "tier": "string — acute_today | trend_this_week",
      "status": "string — sent | viewed | acted | dismissed",
      "risk_score": "integer — skor risk_scores pemicu (0 jika tak ada)",
      "risk_status": "string — aman | waswas | bahaya ('' jika tak ada)",
      "sent_at": "string — ISO 8601",
      "viewed_at": "string | null — ISO 8601",
      "acted_at": "string | null — ISO 8601",
      "created_at": "string — ISO 8601"
    }
  ],
  "paging": { "page": 1, "size": 20, "total_item": 1, "total_page": 1 }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 401 | Token nakes tidak ada / invalid |
| 500 | Kegagalan server |

**Catatan:** Hanya eskalasi milik faskes JWT yang muncul (tenant isolation). `patient_name`, `risk_score`, dan `risk_status` di-JOIN-resolve dari `patients` dan `risk_scores` pemicu. Saat eskalasi `acute_today` dibuat, sistem juga mengirim notifikasi keluar: satu baris inbox in-app ke pasien (`patient_notifications` tipe `escalation`) dan blast WhatsApp ke nakes, pasien, serta pendamping (bila ada nomornya). Tiap pengiriman WA dicatat di tabel `notifications` (`message_type=escalation`, `status` queued→sent/failed, `escalation_id` terisi). Bila jumlah eskalasi harian seorang nakes melewati `ALERT_BUDGET` (default 20), blast WA dilewati tetapi eskalasi & inbox tetap dibuat.

---

### `PATCH /api/v1/nakes/escalations/{id}/view`

**Deskripsi:** Menandai eskalasi sebagai `viewed` (idempoten — aman dipanggil berulang; jika sudah viewed/acted/dismissed maka no-op). `faskes_id` dari JWT.

**Auth:** Bearer JWT (nakes)

**Request**

Path params:
| Param | Tipe | Keterangan |
|---|---|---|
| `id` | string (UUID) | ID eskalasi |

_(tanpa body)_

**Response — 200 OK**
```json
{ "message": "status eskalasi berhasil diperbarui", "data": null }
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | `id` kosong |
| 401 | Token nakes tidak ada / invalid |
| 404 | Eskalasi tidak ditemukan / milik faskes lain |
| 500 | Kegagalan server |

**Catatan:** Eskalasi milik faskes lain dikembalikan sebagai 404 (bukan 403) agar keberadaannya tidak bocor lintas tenant (konsisten dengan endpoint lain).

---

### `PATCH /api/v1/nakes/escalations/{id}/act`

**Deskripsi:** Menandai eskalasi sudah ditindaklanjuti (`acted`). `faskes_id` dari JWT.

**Auth:** Bearer JWT (nakes)

**Request**

Path params:
| Param | Tipe | Keterangan |
|---|---|---|
| `id` | string (UUID) | ID eskalasi |

_(tanpa body)_

**Response — 200 OK**
```json
{ "message": "status eskalasi berhasil diperbarui", "data": null }
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | `id` kosong |
| 401 | Token nakes tidak ada / invalid |
| 404 | Eskalasi tidak ditemukan / milik faskes lain |
| 409 | Eskalasi sudah ditutup (`acted`/`dismissed`) |
| 500 | Kegagalan server |

---

### `PATCH /api/v1/nakes/escalations/{id}/dismiss`

**Deskripsi:** Menandai eskalasi diabaikan (`dismissed`). `faskes_id` dari JWT.

**Auth:** Bearer JWT (nakes)

**Request**

Path params:
| Param | Tipe | Keterangan |
|---|---|---|
| `id` | string (UUID) | ID eskalasi |

_(tanpa body)_

**Response — 200 OK**
```json
{ "message": "status eskalasi berhasil diperbarui", "data": null }
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | `id` kosong |
| 401 | Token nakes tidak ada / invalid |
| 404 | Eskalasi tidak ditemukan / milik faskes lain |
| 409 | Eskalasi sudah ditutup (`acted`/`dismissed`) |
| 500 | Kegagalan server |

---

### `PATCH /api/v1/nakes/escalations/{id}/feedback`

**Deskripsi:** Nakes menilai apakah eskalasi tepat atau tidak (`accurate`/`inaccurate`) — label emas untuk perbaikan model. Boleh diisi tanpa memandang status lifecycle. `faskes_id` & `nakes_id` dari JWT.

**Auth:** Bearer JWT (nakes)

**Request**

Path params:
| Param | Tipe | Keterangan |
|---|---|---|
| `id` | string (UUID) | ID eskalasi |

Body:
```json
{ "feedback": "string — accurate | inaccurate (wajib)" }
```

**Response — 200 OK**
```json
{ "message": "feedback eskalasi berhasil disimpan", "data": null }
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | `id` kosong / `feedback` bukan `accurate`/`inaccurate` |
| 401 | Token nakes tidak ada / invalid |
| 404 | Eskalasi tidak ditemukan / milik faskes lain |
| 500 | Kegagalan server |

---

### Dashboard Pasien

---

### `GET /api/v1/patients/dashboard`

**Deskripsi:** Mengambil data agregat untuk layar utama (home) Patient App (mobile): profil pasien, status risiko terbaru, pengukuran terakhir (gula darah & tekanan darah), status logging harian (streak), dan rekomendasi. Semua data di-scope ke pasien yang sedang login — `patient_id` diambil dari JWT, tidak pernah dari request.

**Auth:** Bearer JWT (patient)

**Request**

_(tidak ada path param, query param, atau body)_

**Response — 200 OK**
```json
{
  "message": "dashboard pasien berhasil diambil",
  "data": {
    "profile": {
      "full_name": "string — nama lengkap pasien",
      "age": 58,
      "disease_type": "string — diabetes_t2 | hypertension | both",
      "companion_name": "string — nama pendamping, kosong jika tidak ada",
      "companion_phone": "string — nomor WA pendamping, kosong jika tidak ada",
      "assigned_nakes_name": "string — nama dokter penanggung jawab, kosong jika nakes tidak ditemukan"
    },
    "risk": {
      "score": 72,
      "risk_label": "string — kritis | sedang | rendah",
      "status": "string — bahaya | waswas | aman",
      "main_factor": "string — label Indonesia faktor SHAP tertinggi, kosong jika belum ada skor",
      "scored_at": "string | null — ISO 8601 timestamp skor terakhir, null jika belum ada skor"
    },
    "latest_measurements": {
      "glucose": { "value": 180, "measured_at": "string — ISO 8601" },
      "blood_pressure": { "systolic": 140, "diastolic": 90, "measured_at": "string — ISO 8601" }
    },
    "logging": {
      "logged_today": true,
      "streak_days": 5
    },
    "recommendations": ["string — kalimat anjuran berbahasa Indonesia"]
  }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 401 | Token pasien tidak ada / invalid / expired |
| 500 | Kegagalan server |

**Catatan:**
- `risk.score` adalah `health_score` (0–100, **TINGGI = sehat**). `risk_label` diturunkan terbalik: `≤ 40` → `kritis`, `41–70` → `sedang`, `> 70` → `rendah` (konsisten dengan dashboard nakes).
- `risk.status` adalah nilai enum `risk_scores.status`. `main_factor` = elemen pertama `top_factors` (SHAP tertinggi) diterjemahkan ke label Indonesia.
- Pasien tanpa risk score: `risk.score: 0`, `risk_label: "rendah"`, `status: "aman"`, `main_factor: ""`, `scored_at: null`.
- `latest_measurements.glucose` dan `blood_pressure` bernilai `null` jika pasien belum punya log metrik tersebut. Tekanan darah dibaca dari `health_logs.value_jsonb` (`{"systolic", "diastolic"}`).
- `logging.logged_today` = ada `health_log` dengan `measured_at::date = hari ini`. `streak_days` = jumlah hari berturut-turut yang punya log, dihitung mundur dari hari terakhir yang punya log (hari ini jika ada, kalau tidak kemarin); `0` jika log terakhir lebih lama dari kemarin.
- `recommendations` diturunkan dari hingga 3 faktor SHAP teratas (`top_factors`) yang dipetakan ke kalimat anjuran statis. Jika belum ada risk score, berisi satu pesan onboarding generik.
- **Catatan data:** `latest_measurements` dan `logging` membaca `health_logs`. Jalur input pasien sudah tersedia via `POST /api/v1/patients/health-logs`; section ini tetap mengembalikan `null`/`0` untuk pasien yang belum punya log. Ingestion via webhook WhatsApp belum dibangun.
- `profile.assigned_nakes_name` di-resolve dari `assigned_nakes_id` pasien; bila nakes tidak ditemukan, field dikembalikan sebagai `""` dan dashboard tetap dikembalikan (non-fatal). Untuk info dokter lengkap, gunakan `GET /api/v1/patients/assigned-nakes`.

---

### `GET /api/v1/patients/summary`

**Deskripsi:** Ringkasan kesehatan pasien yang sedang login pada window 7/14/30 hari — gabungan angka agregat (dihitung backend dari `health_logs`) + narasi (AI/Gemini) bernada awam & memotivasi. Window hanya tersedia bila riwayat data pasien menutupinya.

**Auth:** Bearer JWT (patient)

**Request**

Query params:
| Param | Tipe | Wajib | Default | Keterangan |
|---|---|---|---|---|
| `window` | integer | Tidak | 7 | Salah satu dari `7`, `14`, `30` (hari) |

**Response — 200 OK (data cukup)**
```json
{
  "message": "ringkasan kesehatan berhasil diambil",
  "data": {
    "window": 7,
    "available": true,
    "available_windows": [7, 14, 30],
    "period": { "start": "2026-06-24", "end": "2026-06-30" },
    "coverage": { "logged_days": 5, "window_days": 7, "streak_days": 3 },
    "aggregates": {
      "glucose": { "avg_mgdl": 142.5, "min_mgdl": 98, "max_mgdl": 210, "count": 6 },
      "blood_pressure": { "avg_systolic": 134.2, "avg_diastolic": 85.1, "count": 5 },
      "med_adherence": { "adherence_rate_pct": 80, "count": 5 },
      "nutrition": { "avg_kcal_per_day": 1850.4, "avg_carbs_g_per_day": 210.3, "avg_sodium_mg_per_day": 1200, "meal_count": 8 },
      "activity": { "avg_minutes_per_day": 25, "total_minutes": 75, "count": 3 },
      "sleep": { "avg_hours": 6.5, "count": 4 },
      "stress": { "avg_level": 4.2, "count": 4 },
      "weight": { "start_kg": 70.5, "latest_kg": 70.1, "delta_kg": -0.4, "count": 2 }
    },
    "risk": { "score": 72, "status": "waswas", "scored_at": "2026-06-30T01:00:00Z" },
    "narrative": "string — ringkasan 3-5 kalimat dari Gemini, sapaan 'Anda'",
    "generated_at": "2026-06-30T08:40:00+07:00"
  }
}
```

**Response — 200 OK (data belum cukup untuk window diminta)**
```json
{
  "message": "ringkasan kesehatan berhasil diambil",
  "data": {
    "window": 30,
    "available": false,
    "available_windows": [7],
    "history_days": 9,
    "message": "Data Anda baru mencakup 9 hari, sedangkan ringkasan 30 hari membutuhkan minimal 30 hari pencatatan. Terus catat kondisi harian agar ringkasan ini tersedia.",
    "narrative": "",
    "generated_at": "2026-06-30T08:40:00+07:00"
  }
}
```
> Bila pasien belum punya log sama sekali: `available_windows: []`, `history_days: 0`, dan `message` mengajak mulai mencatat. `history_days` juga selalu ada pada response `available:true`.

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | `window` bukan 7/14/30 |
| 401 | Token pasien tidak ada / invalid / expired |
| 500 | Kegagalan server |

**Catatan:**
- Sama dengan versi nakes, hanya berbeda nada narasi (awam/memotivasi) dan sumber identitas: `patient_id` diambil dari JWT (data sendiri).
- Tiap sub-objek `aggregates` `null` bila tak ada data metrik tsb; `period`/`coverage`/`aggregates` dihilangkan saat `available:false`.
- `available_windows`: window `w` valid bila rentang hari log pertama s.d. hari ini ≥ `w`. Hari di-bucket pada zona **Asia/Jakarta (WIB)**.
- **Efek samping:** cache Redis key `summary:patient:{patientId}:{window}:{YYYY-MM-DD WIB}` (TTL ~24 jam) + panggilan Gemini saat cache miss. Bila Gemini gagal: tetap 200, angka agregat utuh, `narrative` fallback (tidak di-cache).

---

### `GET /api/v1/patients/assigned-nakes`

**Deskripsi:** Mengambil informasi dokter/nakes penanggung jawab yang ditugaskan untuk pasien yang sedang login. Digunakan oleh layar Dokter di Patient App. `patient_id` diambil dari JWT.

**Auth:** Bearer JWT (patient)

**Request**

_(tidak ada path param, query param, atau body)_

**Response — 200 OK**
```json
{
  "message": "informasi dokter berhasil diambil",
  "data": {
    "full_name": "string — nama lengkap nakes",
    "specialization": "string — spesialisasi dokter, kosong jika belum diisi",
    "hospital": "string — nama rumah sakit / klinik, kosong jika belum diisi",
    "whatsapp_phone": "string — nomor WA nakes (format internasional)",
    "wa_link": "string — link wa.me ke nomor WA dokter (https://wa.me/{phone}); kosong jika phone_number dokter tidak tersimpan",
    "schedule": [
      { "days": "string — contoh: Senin - Jumat", "time": "string — contoh: 08.00 - 14.00" }
    ]
  }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 401 | Token pasien tidak ada / invalid / expired |
| 404 | Dokter penanggung jawab tidak ditemukan (data integrity issue) |
| 500 | Kegagalan server |

**Catatan:** `specialization`, `hospital`, dan `schedule` bersumber dari kolom yang ditambahkan migration `000008_patient_mobile_features` ke tabel `nakes`. Kolom ini nullable — field dikembalikan sebagai `""` / `[]` jika belum diisi admin faskes. `schedule` adalah array JSON yang disimpan di kolom `jsonb` tabel `nakes`. `wa_link` dibuild dari `phone_number` dokter yang dinormalisasi ke format internasional — link langsung dibuka WhatsApp klien untuk menghubungi dokter, bukan melalui bot.

---

### `POST /api/v1/patients/consultations`

**Deskripsi:** Pasien mengirim keluhan/pertanyaan ke dokter penanggung jawab. Keluhan disimpan ke tabel `consultations` dengan status `open`. `patient_id` diambil dari JWT.

**Auth:** Bearer JWT (patient)

**Request**

Body:
```json
{
  "complaint_since":  "string — kapan keluhan mulai dirasakan (wajib, 1–500 karakter, contoh: \"Sejak kemarin, 3 hari yang lalu\")",
  "complaint_type":   "string — jenis keluhan/sakit (wajib, 1–500 karakter, contoh: \"Pusing, nyeri perut\")",
  "complaint_detail": "string — detail keluhan (wajib, 1–2000 karakter)"
}
```

**Response — 201 Created**
```json
{
  "message": "keluhan berhasil dikirim",
  "data": {
    "id":               "string — UUID konsultasi",
    "patient_id":       "string — UUID pasien",
    "complaint_since":  "string",
    "complaint_type":   "string",
    "complaint_detail": "string",
    "status":           "string — selalu open saat baru dibuat",
    "nakes_note":       "null",
    "replied_at":       "null",
    "created_at":       "string — ISO 8601 timestamp"
  }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | Body tidak valid / salah satu field wajib kosong atau melebihi batas karakter |
| 401 | Token pasien tidak ada / invalid / expired |
| 500 | Kegagalan server |

**Catatan:** MVP: keluhan disimpan ke tabel `consultations`, notifikasi WA ke nakes belum diimplementasi.

---

### `GET /api/v1/patients/consultations`

**Deskripsi:** Mengambil seluruh riwayat konsultasi pasien yang sedang login, termasuk balasan dokter jika sudah ada. Diurutkan dari yang terbaru. `patient_id` diambil dari JWT.

**Auth:** Bearer JWT (patient)

**Request**

_(tidak ada path param, query param, atau body)_

**Response — 200 OK**
```json
{
  "message": "daftar konsultasi berhasil diambil",
  "data": [
    {
      "id":               "string — UUID konsultasi",
      "patient_id":       "string — UUID pasien",
      "complaint_since":  "string — kapan keluhan mulai",
      "complaint_type":   "string — jenis keluhan",
      "complaint_detail": "string — detail keluhan",
      "status":           "string — open | replied",
      "nakes_note":       "string | null — balasan dokter, null jika belum dibalas",
      "replied_at":       "string | null — ISO 8601 timestamp balasan, null jika belum",
      "created_at":       "string — ISO 8601 timestamp"
    }
  ]
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 401 | Token pasien tidak ada / invalid / expired |
| 500 | Kegagalan server |

---

### `GET /api/v1/patients/notifications`

**Deskripsi:** Mengambil seluruh notifikasi inbox in-app milik pasien yang sedang login, diurutkan dari yang terbaru. Inbox berisi dua tipe: `consultation_reply` (dibuat backend saat nakes membalas konsultasi) dan `daily_reminder` (dibuat worker saat pasien belum mencatat data harian). `patient_id` diambil dari JWT.

**Auth:** Bearer JWT (patient)

**Request**

_(tidak ada path param, query param, atau body)_

**Response — 200 OK**
```json
{
  "message": "notifikasi berhasil diambil",
  "data": [
    {
      "id":         "string — UUID notifikasi",
      "type":       "string — consultation_reply | daily_reminder",
      "title":      "string — judul siap-tampil (mis. \"Balasan dari dokter\")",
      "body":       "string — isi siap-tampil (mis. catatan balasan dokter)",
      "is_read":    false,
      "read_at":    "string | null — ISO 8601 timestamp saat dibaca, null jika belum",
      "created_at": "string — ISO 8601 timestamp waktu notifikasi dibuat",
      "data": {
        "consultation_id": "string | null — UUID konsultasi (hanya untuk consultation_reply)",
        "nakes_name":      "string | null — nama nakes yang membalas (hanya consultation_reply)"
      }
    }
  ]
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 401 | Token pasien tidak ada / invalid / expired |
| 500 | Kegagalan server |

**Catatan:** Notifikasi inbox disimpan di tabel **`patient_notifications`** (bukan lagi `notifications` — tabel itu kini murni log transport WA/SMS). `title`/`body` sudah dirender server-side; objek `data` memuat field spesifik-tipe untuk keperluan deep-link. State baca dikelola via `read_at` (lihat endpoint mark-as-read di bawah).

---

### `GET /api/v1/patients/notifications/unread-count`

**Deskripsi:** Mengembalikan jumlah notifikasi inbox yang belum dibaca milik pasien yang sedang login. Dipakai untuk badge di Patient App. `patient_id` diambil dari JWT.

**Auth:** Bearer JWT (patient)

**Request**

_(tidak ada path param, query param, atau body)_

**Response — 200 OK**
```json
{
  "message": "jumlah notifikasi belum dibaca",
  "data": { "unread_count": 3 }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 401 | Token pasien tidak ada / invalid / expired |
| 500 | Kegagalan server |

---

### `PATCH /api/v1/patients/notifications/{id}/read`

**Deskripsi:** Menandai satu notifikasi inbox sebagai sudah dibaca. Idempoten — menandai notifikasi yang sudah terbaca tetap mengembalikan 200. `patient_id` diambil dari JWT.

**Auth:** Bearer JWT (patient)

**Request**

Path params:
| Param | Tipe | Keterangan |
|---|---|---|
| `id` | string (UUID) | ID notifikasi yang ditandai sudah dibaca |

**Response — 200 OK**
```json
{
  "message": "notifikasi ditandai sudah dibaca",
  "data": null
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | `id` tidak diisi |
| 401 | Token pasien tidak ada / invalid / expired |
| 404 | Notifikasi tidak ditemukan atau bukan milik pasien yang sedang login |
| 500 | Kegagalan server |

**Catatan:** Notifikasi milik pasien lain dikembalikan sebagai 404 (bukan 403) agar keberadaannya tidak bocor lintas pasien — konsisten dengan konvensi tenant isolation di kontrak ini.

---

### `POST /api/v1/patients/notifications/read-all`

**Deskripsi:** Menandai seluruh notifikasi inbox yang belum dibaca milik pasien yang sedang login sebagai sudah dibaca. `patient_id` diambil dari JWT.

**Auth:** Bearer JWT (patient)

**Request**

_(tidak ada path param, query param, atau body)_

**Response — 200 OK**
```json
{
  "message": "semua notifikasi ditandai sudah dibaca",
  "data": { "updated_count": 5 }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 401 | Token pasien tidak ada / invalid / expired |
| 500 | Kegagalan server |

---

### `POST /api/v1/patients/device-tokens`

**Deskripsi:** Mendaftarkan (atau memperbarui) satu device push token FCM milik pasien yang sedang login, dipanggil Patient App setiap kali token FCM baru diterbitkan (mis. saat login atau saat token di-refresh oleh Firebase). Upsert by token — token yang sama yang didaftarkan ulang tidak membuat baris duplikat. `patient_id` diambil dari JWT, bukan dari body.

**Auth:** Bearer JWT (patient)

**Request**

Body:
```json
{
  "token":    "string — FCM registration token",
  "platform": "string — android | ios"
}
```

**Response — 200 OK**
```json
{
  "message": "device token berhasil didaftarkan",
  "data": null
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | `token` kosong / `platform` bukan `android`/`ios` |
| 401 | Token pasien tidak ada / invalid / expired |
| 500 | Kegagalan server |

**Catatan:** Disimpan di tabel `device_push_tokens`. Dipakai sebagai target pengiriman push notification (FCM) dari backend saat ada event yang perlu dipush ke Patient App (mis. eskalasi, balasan konsultasi). Mendukung multi-device: satu pasien boleh punya banyak token aktif sekaligus, semuanya menerima push.

---

### `DELETE /api/v1/patients/device-tokens`

**Deskripsi:** Menonaktifkan (deregister) satu device push token milik pasien yang sedang login — dipanggil Patient App saat logout agar device tersebut berhenti menerima push notification. `patient_id` diambil dari JWT.

**Auth:** Bearer JWT (patient)

**Request**

Body:
```json
{
  "token": "string — FCM registration token yang akan dinonaktifkan"
}
```

**Response — 200 OK**
```json
{
  "message": "device token berhasil dihapus",
  "data": null
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | `token` kosong |
| 401 | Token pasien tidak ada / invalid / expired |
| 500 | Kegagalan server |

**Catatan:** Deregistrasi menonaktifkan (bukan menghapus baris) token pada tabel `device_push_tokens`, konsisten dengan pola soft-deactivate yang dipakai di tabel lain pada kontrak ini. Idempoten — token tidak ditemukan atau bukan milik pasien ini tetap 200 (mobile tidak perlu menangani error khusus di jalur logout). Scoped by `patient_id` dari JWT.

---

### `POST /api/v1/patients/records`

**Deskripsi:** Pasien menyimpan catatan harian dari form native Patient App — satu request untuk semua metrik yang diisi. Setiap metrik yang diisi menghasilkan satu baris di `health_logs`. Minimal satu metrik harus diisi. `patient_id` diambil dari JWT; `source` selalu `app`.

**Auth:** Bearer JWT (patient)

**Request**

Body (semua field opsional kecuali `recorded_at`, tapi minimal satu metrik harus diisi):
```json
{
  "blood_sugar": "number | null — gula darah (mg/dL), range 20–600",
  "systolic": "int | null — tekanan darah sistolik, range 40–300",
  "diastolic": "int | null — tekanan darah diastolik, range 20–200; wajib jika systolic diisi",
  "weight": "number | null — berat badan (kg), range 1–500",
  "medicine_taken": "bool | null — true = kepatuhan 100%, false = 0%",
  "meals": "string — catatan makanan (maks 500 char), dikosongkan jika tidak diisi",
  "recorded_at": "string — wajib, RFC3339 / ISO 8601, waktu pengukuran, tidak boleh di masa depan"
}
```

**Response — 201 Created**
```json
{
  "message": "catatan harian berhasil disimpan",
  "data": {
    "recorded_at": "string — ISO 8601 timestamp pencatatan",
    "created": ["string — daftar metric_type yang berhasil disimpan"]
  }
}
```

Contoh `created`: `["glucose", "bp", "weight", "med_adherence", "food"]`

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | `recorded_at` tidak ada / bukan RFC3339 / di masa depan; nilai metrik di luar range; `systolic` diisi tanpa `diastolic` (atau sebaliknya); tidak ada satupun metrik yang diisi |
| 401 | Token pasien tidak ada / invalid / expired |
| 500 | Kegagalan server |

**Catatan:**
- `systolic` dan `diastolic` harus diisi bersama — satu tanpa yang lain dikembalikan 400.
- `weight` memerlukan migration `000008_patient_mobile_features` (menambah `weight` ke enum `health_metric`).
- Tidak ada idempotency key — endpoint ini dirancang untuk form explicit submit, bukan retry otomatis. Untuk input per-metrik dengan idempotency, gunakan `POST /api/v1/patients/health-logs`.
- Setiap metrik yang berhasil disimpan menghasilkan satu baris di `health_logs` (tabel insert-only). `logged_by = patient`, `source = app`.

---

### `GET /api/v1/patients/records/history`

**Deskripsi:** Mengambil riwayat catatan harian pasien untuk grafik di Patient App. Untuk setiap hari, dikembalikan nilai terbaru per metrik (glucose, bp, weight) dan health score terbaru pada tanggal tersebut jika tersedia. `patient_id` diambil dari JWT.

**Auth:** Bearer JWT (patient)

**Request**

Query params:
| Param | Tipe | Wajib | Default | Keterangan |
|---|---|---|---|---|
| `limit` | integer | Tidak | 7 | Jumlah hari yang dikembalikan (maks efektif 90, default 7) |

**Response — 200 OK**
```json
{
  "message": "riwayat catatan berhasil diambil",
  "data": [
    {
      "date": "string — YYYY-MM-DD",
      "blood_sugar": "number | null — gula darah terbaru pada hari itu",
      "systolic": "int | null — sistolik terbaru pada hari itu",
      "diastolic": "int | null — diastolik terbaru pada hari itu",
      "weight": "number | null — berat badan terbaru pada hari itu",
      "health_score": "int | null — health score terbaru yang dihitung pada tanggal itu"
    }
  ]
}
```

Array diurutkan dari hari terbaru ke terlama. Field metrik bernilai `null` jika pasien tidak mencatat metrik tersebut pada hari itu. `health_score` bernilai `null` jika belum ada baris `risk_scores` pada tanggal tersebut.

**Response — Error**
| Status | Kondisi |
|---|---|
| 401 | Token pasien tidak ada / invalid / expired |
| 500 | Kegagalan server |

**Catatan:** Query ini hanya membaca hari yang memiliki minimal satu log dari metrik `glucose`, `bp`, atau `weight` — hari tanpa log sama sekali tidak muncul. `health_score` diambil dari `risk_scores.score` terbaru untuk `daily_features.feature_date` pada tanggal yang sama. `limit > 90` diclamp ke 7 oleh usecase.

---

### `GET /api/v1/patients/records/today-status`

**Deskripsi:** Mengecek apakah pasien sudah mengisi data harian **hari ini** (zona WIB). Dipakai Patient App untuk memunculkan pop-up pengingat ketika pasien lupa mengisi (`logged_today == false`). Endpoint ringan — hanya mengembalikan satu boolean + tanggal. `patient_id` diambil dari JWT.

**Auth:** Bearer JWT (patient)

**Request**

Tidak ada body / query param.

**Response — 200 OK**
```json
{
  "message": "status input harian berhasil diambil",
  "data": {
    "logged_today": "bool — true jika input terakhir jatuh di tanggal (WIB) hari ini",
    "days_since_last_log": "int | null — jumlah hari (WIB) sejak input terakhir; 0 = hari ini, 1 = kemarin, dst. null jika pasien belum pernah mengisi",
    "last_logged_at": "string | null — ISO 8601, waktu input terakhir; null jika belum pernah",
    "date": "string — YYYY-MM-DD, tanggal hari ini menurut WIB (Asia/Jakarta)"
  }
}
```

**Response — Error**
| Status | Kondisi |
|---|---|
| 401 | Token pasien tidak ada / invalid / expired |
| 500 | Kegagalan server |

**Catatan:** "Sudah isi" didefinisikan longgar — minimal satu `health_log` dengan metrik apa pun pada hari ini sudah dihitung `true`. Penentuan "hari ini" dan selisih hari memakai zona `Asia/Jakarta`, bukan timezone server, agar benar di sekitar tengah malam. `days_since_last_log` memungkinkan mobile menampilkan "kamu sudah X hari tidak mengisi" untuk kasus lupa lebih dari satu hari; `null` (bersama `last_logged_at` null) berarti pasien belum pernah input sama sekali. Nilai `logged_today` konsisten dengan `logging.logged_today` pada `GET /api/v1/patients/dashboard`.

---

### `GET /api/v1/patients/records/logged-today`

**Deskripsi:** Mengecek apakah pasien sudah mengisi data harian **hari ini** (zona WIB) dan mengembalikan satu nilai boolean mentah. Gunakan endpoint ini ketika mobile hanya butuh jawaban ya/tidak tanpa payload tambahan. Untuk konteks lebih lengkap (kapan terakhir input, sudah berapa hari), gunakan `GET /api/v1/patients/records/today-status`. `patient_id` diambil dari JWT.

**Auth:** Bearer JWT (patient)

**Request**

Tidak ada body / query param.

**Response — 200 OK**
```json
{
  "message": "status input harian berhasil diambil",
  "data": true
}
```

`data` adalah `bool`:
- `true` — pasien memiliki minimal satu `health_log` pada hari ini (WIB / Asia Jakarta).
- `false` — belum ada input hari ini, atau pasien belum pernah mengisi sama sekali.

**Response — Error**
| Status | Kondisi |
|---|---|
| 401 | Token pasien tidak ada / invalid / expired |
| 500 | Kegagalan server |

**Catatan:** Penentuan "hari ini" memakai zona `Asia/Jakarta` (WIB, UTC+7). Logika identik dengan field `logged_today` pada `GET /api/v1/patients/records/today-status`; endpoint ini adalah bentuk yang lebih ringkas dari yang sama.

---

### `GET /api/v1/patients/baseline/history`

**Deskripsi:** Pasien melihat **progress baseline klinis dirinya sendiri** (metrik kunci) dari waktu ke waktu, terbaru-dulu. `patient_id` diambil dari JWT — pasien hanya melihat baseline miliknya.

**Auth:** Bearer JWT (patient)

**Request**

Query params:
| Param | Tipe | Wajib | Keterangan |
|---|---|---|---|
| `page` | int | tidak | Default 1 |
| `size` | int | tidak | Default 20, maks 100 |

**Response — 200 OK**

Sama bentuk dengan `GET /api/v1/faskes/patients/{id}/baseline/history` (array `BaselineHistoryItem` + `paging`): `id`, `recorded_at`, `recorded_by_nakes_name`, `notes`, `bmi`, `bmi_category`, `systolic_bp_mmhg`, `diastolic_bp_mmhg`, `hypertension_status`, `fasting_glucose_mgdl`, `hba1c_pct`, `diabetes_status`, `total_cholesterol_mgdl`, `hdl_mgdl`, `ldl_mgdl`, `triglycerides_mgdl`, `cvd_risk_10yr_pct`, `cvd_risk_category`, `egfr`, `uacr`.

**Response — Error**
| Status | Kondisi |
|---|---|
| 401 | Token pasien tidak ada / invalid / expired |
| 500 | Kegagalan server |

**Catatan:** Shape response identik dengan versi faskes agar konsisten. Diurutkan `recorded_at` menurun. Pasien tanpa baseline mengembalikan `data: []`.

---

### Health Logs (input harian)

---

### `POST /api/v1/patients/health-logs`

**Deskripsi:** Pasien (Patient App / mobile) mencatat **satu** pengukuran kesehatan harian
(satu metrik per request → satu baris di `health_logs`). `patient_id` diambil dari JWT,
tidak pernah dari body. `logged_by` selalu `patient` dan `source` selalu `web` (di-set
server-side). `measured_at` dikirim client supaya entri yang sempat tertunda tetap tercatat
dengan waktu pengukuran asli.

**Auth:** Bearer JWT (patient)

**Request**

Headers:
| Header | Wajib | Keterangan |
|---|---|---|
| `Authorization` | ya | `Bearer <access_token>` (patient) |
| `Idempotency-Key` | ya | UUID dibuat client; dedupe double-tap. Request kedua dengan key sama dalam 5 menit mengembalikan response yang sama tanpa insert dobel. |

Body (bentuk nilai tergantung `metric_type`):
```json
{
  "metric_type": "string — glucose | bp | med_adherence | food | activity | sleep | stress | smoking | alcohol | weight",
  "value_numeric": "number — wajib untuk metrik numerik (semua kecuali bp & food)",
  "systolic": "int — wajib untuk metric_type=bp",
  "diastolic": "int — wajib untuk metric_type=bp",
  "value_text": "string — wajib untuk metric_type=food (maks 500 char)",
  "measured_at": "string — wajib, RFC3339 / ISO 8601, tidak boleh di masa depan"
}
```

Aturan nilai & range per `metric_type`:
| metric_type | field nilai | range | disimpan ke |
|---|---|---|---|
| `glucose` | `value_numeric` | 20–600 (mg/dL) | `value_numeric` |
| `bp` | `systolic`, `diastolic` | sys 40–300, dia 20–200, sys > dia | `value_jsonb` `{"systolic":N,"diastolic":N}` |
| `med_adherence` | `value_numeric` | 0–100 (%) | `value_numeric` |
| `food` | `value_text` | non-kosong, maks 500 char | `value_text` |
| `activity` | `value_numeric` | 0–1440 (menit) | `value_numeric` |
| `sleep` | `value_numeric` | 0–24 (jam) | `value_numeric` |
| `stress` | `value_numeric` | 1–10 | `value_numeric` |
| `smoking` | `value_numeric` | 0–200 (batang) | `value_numeric` |
| `alcohol` | `value_numeric` | 0–100 (unit) | `value_numeric` |
| `weight` | `value_numeric` | 1–500 (kg) | `value_numeric` |

Contoh — glucose:
```json
{ "metric_type": "glucose", "value_numeric": 180, "measured_at": "2026-06-28T07:00:00Z" }
```

Contoh — tekanan darah (bp):
```json
{ "metric_type": "bp", "systolic": 140, "diastolic": 90, "measured_at": "2026-06-28T07:05:00Z" }
```

**Response — 201 Created**
```json
{
  "message": "data harian berhasil dicatat",
  "data": {
    "id": "string — uuid health log",
    "patient_id": "string — uuid",
    "metric_type": "string",
    "value_numeric": 180,
    "value_text": "string — hanya untuk food",
    "blood_pressure": { "systolic": 140, "diastolic": 90 },
    "measured_at": "string — ISO 8601",
    "logged_by": "string — selalu patient",
    "source": "string — selalu web",
    "created_at": "string — ISO 8601"
  }
}
```
> `value_numeric`, `value_text`, dan `blood_pressure` saling eksklusif sesuai `metric_type`
> (field yang tidak relevan dihilangkan dari JSON).

**Response — Error**
| Status | Kondisi |
|---|---|
| 400 | `Idempotency-Key` header tidak ada; body gagal validasi struct; nilai per-metric tidak valid / di luar range; `measured_at` bukan RFC3339 atau di masa depan |
| 401 | Token pasien tidak ada / invalid / expired |
| 409 | Request dengan `Idempotency-Key` yang sama masih diproses (in-flight) |
| 429 | Terlalu banyak submission dalam waktu singkat (rate limit, maks 30 / menit per pasien) |
| 500 | Kegagalan server |

**Catatan:**
- Tabel `health_logs` insert-only — tidak ada UPDATE/DELETE. Satu request = satu baris.
- `logged_by` & `source` di-set server-side (`patient` / `web`); nilai di body diabaikan.
  Pendamping/keluarga tetap input lewat WhatsApp, bukan endpoint ini.
- Konvensi `bp`: kedua angka disimpan dalam **satu baris** di `value_jsonb`
  (`{"systolic":N,"diastolic":N}`), bukan dua log terpisah — konsisten dengan cara dashboard
  pasien membacanya.
- Idempotency: key disimpan di Redis (`idempotency:{key}`, TTL 5 menit, lihat `docs/redis.md`).
  Request ulang dengan key sama dalam window mengembalikan `id` yang sama tanpa insert dobel.
- Endpoint ini mengisi `health_logs` yang dibaca `GET /api/v1/patients/dashboard`
  (`latest_measurements`, `logging.logged_today`, `streak_days`).

### Lab Results

_(belum ada)_

### Risk Scores

_(belum ada)_

### Escalations

_(belum ada)_

### Notifications

_(belum ada)_

### Webhook WhatsApp — Input Data Harian via Pesan Teks

**Deskripsi:** Pasien atau pendamping mengirim pesan teks ke nomor WA bot Sehatiku untuk mencatat data kesehatan harian. Bot membalas konfirmasi jika berhasil, atau panduan format jika pesan tidak dikenali. Tidak ada endpoint HTTP — ini adalah kanal inbound dari whatsmeow event handler.

**Auth:** Tidak ada (diidentifikasi berdasarkan `phone_number` atau `companion_phone` di tabel `patients`)

**Format Pesan yang Didukung (case-insensitive)**

| Metrik | Contoh Pesan | Disimpan sebagai |
|---|---|---|
| Gula darah | `gula 180`, `gds 140`, `gula darah 160` | `metric_type=glucose`, `value_numeric=180` |
| Tekanan darah | `tensi 120/80`, `td 130/85`, `tekanan darah 140/90` | `metric_type=bp`, `value_jsonb={"systolic":120,"diastolic":80}` |
| Kepatuhan obat | `obat ya`, `minum obat`, `sudah minum obat` | `metric_type=med_adherence`, `value_numeric=100` |
| Tidak minum obat | `obat tidak`, `tidak minum obat`, `lupa obat` | `metric_type=med_adherence`, `value_numeric=0` |
| Makanan | `makan nasi goreng`, `sarapan bubur` | `metric_type=food`, `value_text="makan nasi goreng"` |
| Olahraga | `olahraga 30 menit`, `jalan 45`, `lari 20 menit` | `metric_type=activity`, `value_numeric=30` |
| Tidur | `tidur 7 jam`, `tidur 6.5 jam` | `metric_type=sleep`, `value_numeric=7` |
| Stres | `stres 4`, `stress 3` | `metric_type=stress`, `value_numeric=4` |
| Berat badan | `berat 65 kg`, `bb 70`, `berat badan 68` | `metric_type=weight`, `value_numeric=65` |
| Rokok | `rokok 5 batang`, `merokok 3` | `metric_type=smoking`, `value_numeric=5` |
| Tidak rokok | `tidak rokok`, `berhenti rokok` | `metric_type=smoking`, `value_numeric=0` |
| Alkohol | `alkohol 2 unit`, `minum alkohol 1` | `metric_type=alcohol`, `value_numeric=2` |

**Balasan Bot**

| Kondisi | Balasan |
|---|---|
| Data berhasil disimpan | "✅ Sehatiku — Data Berhasil Dicatat: Gula darah: 180 mg/dL sudah kami simpan." |
| Pesan tidak dikenali | "❓ Sehatiku — panduan format lengkap (semua metrik yang didukung)" |
| Nomor tidak terdaftar | "⚠️ Nomor ini belum terdaftar di Sehatiku. Hubungi faskes Anda." |

**Kolom yang diisi di `health_logs`**

| Kolom | Nilai |
|---|---|
| `logged_by` | `patient` (pasien sendiri) atau `companion` (nomor pendamping) |
| `source` | `whatsapp` |
| `measured_at` | Waktu pesan diterima server (bukan timestamp WA) |

**Catatan:**
- Identifikasi pengirim: server mencari `phone_number = senderPhone` di tabel `patients` (pasien sendiri), lalu `companion_phone = senderPhone` (pendamping). Hanya pasien berstatus `active` yang dikenali.
- Satu pesan = satu metrik. Pesan multi-metrik (mis. "gula 180 tensi 120/80") tidak didukung di MVP — hanya metrik pertama yang dikenali yang diproses.
- Makanan di-enrich via ML `/extract` (NER+TKPI) secara best-effort — jika ML tidak tersedia, makanan tetap disimpan teks-saja (`value_jsonb` null) dan request tidak gagal.
- Pesan non-teks (gambar, audio, sticker) diabaikan tanpa membalas.
- Pesan pertama dari nomor baru (warm-up credential) diproses credential-delivery dulu, kemudian parsing health log tetap dilakukan (tidak di-skip) karena credential sudah dihapus dari Redis setelah terkirim.
- Range validasi sama dengan `POST /api/v1/patients/health-logs` (lihat tabel validasi di endpoint tersebut).
- `measured_at` pada log WA diset ke waktu server saat pesan diproses, bukan ke timestamp pengiriman pesan WA (karena pasien biasanya mengirim langsung saat mengukur).
