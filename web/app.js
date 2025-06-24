function dashboard() {
    return {
        status: 'Yükleniyor...',
        statusClass: 'text-yellow-300',
        stats: {
            total: 0,
            successful: 0,
            failed: 0,
            pending: 0
        },
        messages: [],
        filteredMessages: [],
        filters: {
            direction: '',
            status: '',
            patientId: '',
            messageType: ''
        },
        showModal: false,
        selectedMessage: null,
        refreshInterval: null,

        async init() {
            await this.loadStats();
            await this.loadMessages();
            this.checkSystemStatus();
            
            // Auto refresh every 5 seconds
            this.refreshInterval = setInterval(() => {
                this.refreshData();
            }, 5000);
        },

        async loadStats() {
            try {
                const response = await fetch('/api/stats');
                if (response.ok) {
                    this.stats = await response.json();
                }
            } catch (error) {
                console.error('İstatistik yükleme hatası:', error);
            }
        },

        async loadMessages() {
            try {
                // Load only failed messages from DLQ
                const response = await fetch('/api/messages?status=failed');
                if (response.ok) {
                    const data = await response.json();
                    // Check if it's an error response
                    if (data.message) {
                        this.messages = [];
                    } else {
                        this.messages = data;
                    }
                    this.filterMessages();
                }
            } catch (error) {
                console.error('Mesaj yükleme hatası:', error);
                this.messages = [];
            }
        },

        async checkSystemStatus() {
            try {
                const response = await fetch('/api/health');
                if (response.ok) {
                    const health = await response.json();
                    if (health.status === 'healthy') {
                        this.status = 'Çalışıyor';
                        this.statusClass = 'text-green-300';
                    } else {
                        this.status = 'Sorun Var';
                        this.statusClass = 'text-red-300';
                    }
                } else {
                    this.status = 'Bağlantı Hatası';
                    this.statusClass = 'text-red-300';
                }
            } catch (error) {
                this.status = 'Bağlantı Hatası';
                this.statusClass = 'text-red-300';
            }
        },

        filterMessages() {
            this.filteredMessages = this.messages.filter(msg => {
                if (this.filters.direction && msg.direction !== this.filters.direction) {
                    return false;
                }
                if (this.filters.status && msg.status !== this.filters.status) {
                    return false;
                }
                if (this.filters.patientId && !msg.patient_id?.includes(this.filters.patientId)) {
                    return false;
                }
                if (this.filters.messageType && !msg.message_type?.includes(this.filters.messageType)) {
                    return false;
                }
                return true;
            });
        },

        async refreshData() {
            await this.loadStats();
            await this.loadMessages();
            this.checkSystemStatus();
        },

        viewMessage(message) {
            this.selectedMessage = message;
            this.showModal = true;
        },

        async retryMessage(messageId) {
            try {
                const response = await fetch(`/api/messages/${messageId}/retry`, {
                    method: 'POST'
                });
                if (response.ok) {
                    alert('Mesaj yeniden kuyruğa alındı');
                    await this.refreshData();
                } else {
                    alert('Hata: Mesaj yeniden gönderilemedi');
                }
            } catch (error) {
                console.error('Tekrar deneme hatası:', error);
                alert('Hata: ' + error.message);
            }
        },

        formatDate(timestamp) {
            if (!timestamp) return '-';
            const date = new Date(timestamp);
            return date.toLocaleString('tr-TR');
        },

        getDirectionClass(direction) {
            return direction === 'order' 
                ? 'bg-blue-100 text-blue-800' 
                : 'bg-purple-100 text-purple-800';
        },

        getDirectionText(direction) {
            return direction === 'order' ? 'Order' : 'Report';
        },

        getStatusClass(status) {
            switch (status) {
                case 'forwarded':
                    return 'bg-green-100 text-green-800';
                case 'failed':
                    return 'bg-red-100 text-red-800';
                case 'pending':
                    return 'bg-yellow-100 text-yellow-800';
                default:
                    return 'bg-gray-100 text-gray-800';
            }
        },

        getStatusText(status) {
            switch (status) {
                case 'forwarded':
                    return 'İletildi';
                case 'failed':
                    return 'Başarısız';
                case 'pending':
                    return 'Bekliyor';
                default:
                    return status;
            }
        },

        destroy() {
            if (this.refreshInterval) {
                clearInterval(this.refreshInterval);
            }
        }
    };
}