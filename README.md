# HL7 Replicator

[![Docker Build and Publish](https://github.com/minasoft-technology/hl7-replicator/actions/workflows/docker-publish.yml/badge.svg)](https://github.com/minasoft-technology/hl7-replicator/actions/workflows/docker-publish.yml)
[![Test and Lint](https://github.com/minasoft-technology/hl7-replicator/actions/workflows/test.yml/badge.svg)](https://github.com/minasoft-technology/hl7-replicator/actions/workflows/test.yml)

HL7 Replicator, hastane ağı kısıtlamalarını aşmak için tasarlanmış bir HL7 mesaj yönlendirme ve replikasyon aracıdır. Hastane HIS sisteminden gelen order mesajlarını ZenPACS'a, ZenPACS'tan gelen rapor mesajlarını da hastane HIS sistemine iletir.

## 🎯 Özellikler

- **Çift Yönlü HL7 İletimi**: Order ve rapor mesajlarını iki yönlü olarak iletir
- **Gömülü NATS JetStream**: Mesaj kuyruklama ve güvenilir teslimat için
- **Otomatik Yeniden Deneme**: Başarısız mesajlar için otomatik yeniden deneme mekanizması
- **Web Kontrol Paneli**: Gerçek zamanlı mesaj izleme (Türkçe arayüz)
- **Docker Desteği**: Kolay kurulum ve dağıtım
- **Tek Binary**: Tüm bileşenler tek bir çalıştırılabilir dosyada

## 🚀 Hızlı Başlangıç

### Docker ile Çalıştırma

```bash
# Repository'yi klonlayın
git clone https://github.com/minasoft-technology/hl7-replicator.git
cd hl7-replicator

# Docker ile başlatın
docker-compose up -d
```

### GitHub Container Registry'den Çalıştırma

```bash
# En son image'ı çekin
docker pull ghcr.io/minasoft-technology/hl7-replicator:latest

# Çalıştırın
docker run -d \
  --name hl7-replicator \
  -p 5678:5678 \
  -p 7001:7001 \
  -p 7002:7002 \
  -e HOSPITAL_HIS_HOST=his.hastane.local \
  -e HOSPITAL_HIS_PORT=7200 \
  ghcr.io/minasoft-technology/hl7-replicator:latest
```

### Binary ile Çalıştırma

```bash
# Projeyi derleyin
go build -o hl7-replicator ./cmd/server

# Çalıştırın
./hl7-replicator
```

## 📋 Konfigürasyon

### Ortam Değişkenleri

```bash
# HL7 Dinleme Portları
ORDER_LISTEN_PORT=7001          # HIS'ten order mesajları için
REPORT_LISTEN_PORT=7002         # ZenPACS'tan rapor mesajları için

# ZenPACS Endpoint (Sabit)
ZENPACS_HL7_HOST=194.187.253.34
ZENPACS_HL7_PORT=2575

# Hastane HIS Endpoint
HOSPITAL_HIS_HOST=his.hastane.local
HOSPITAL_HIS_PORT=7200

# Web Dashboard
WEB_PORT=5678

# Veri Depolama
DB_PATH=/data

# Log Seviyesi
LOG_LEVEL=info  # debug, info, warn, error
```

### .env Dosyası Örneği

Proje dizininde `.env` dosyası oluşturun:

```bash
cp .env.example .env
# .env dosyasını düzenleyin
```

## 🏗️ Mimari

```
┌─────────────┐     ┌──────────────────┐     ┌─────────────┐
│ Hastane HIS │────▶│  HL7 Replicator  │────▶│   ZenPACS   │
│             │     │                  │     │ 194.187...  │
│             │◀────│  ┌────────────┐  │◀────│             │
└─────────────┘     │  │NATS Stream │  │     └─────────────┘
                    │  └────────────┘  │
                    │  ┌────────────┐  │
                    │  │Web Dashboard│ │
                    │  └────────────┘  │
                    └──────────────────┘
```

### Bileşenler

1. **HL7 MLLP Sunucular**: Order ve rapor mesajlarını alır
2. **NATS JetStream**: Mesaj kuyruklama ve persistence
3. **Message Forwarders**: Mesajları hedeflerine iletir
4. **Web Dashboard**: İzleme ve yönetim arayüzü

## 📊 Web Dashboard

Web dashboard'a `http://localhost:5678` adresinden erişebilirsiniz.

### Özellikler:
- Gerçek zamanlı mesaj izleme
- Mesaj filtreleme (yön, durum, hasta ID)
- İstatistikler (toplam, başarılı, başarısız)
- Mesaj detaylarını görüntüleme
- Başarısız mesajları yeniden deneme

## 🔧 Geliştirme

### Gereksinimler

- Go 1.21+
- Docker ve Docker Compose (opsiyonel)

### Proje Yapısı

```
hl7-replicator/
├── cmd/server/         # Ana uygulama
├── internal/
│   ├── config/         # Konfigürasyon yönetimi
│   ├── hl7/            # HL7 MLLP sunucu ve client
│   ├── nats/           # Gömülü NATS sunucu
│   ├── consumers/      # JetStream consumer'ları
│   └── web/            # Echo web sunucu
├── web/                # Frontend dosyaları
├── Dockerfile
├── docker-compose.yml
└── README.md
```

### Test Etme

```bash
# HL7 test mesajı gönderme (örnek)
echo -e "\x0BMSH|^~\\&|HIS|HOSPITAL|ZENPACS|MINASOFT|20240101120000||ORM^O01|123456|P|2.5\x1C\x0D" | nc localhost 7001
```

## 📝 Lisans

Bu proje MIT lisansı altında lisanslanmıştır.

## 🤝 Katkıda Bulunma

1. Fork yapın
2. Feature branch oluşturun (`git checkout -b feature/amazing-feature`)
3. Değişikliklerinizi commit edin (`git commit -m 'Add some amazing feature'`)
4. Branch'inizi push edin (`git push origin feature/amazing-feature`)
5. Pull Request açın

## 📞 Destek

Sorularınız veya sorunlarınız için:
- GitHub Issues: [github.com/minasoft-technology/hl7-replicator/issues](https://github.com/minasoft-technology/hl7-replicator/issues)
- Email: support@minasoft.com