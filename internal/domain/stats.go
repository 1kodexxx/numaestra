package domain

// OrderStats — агрегаты по заказам для дашборда админки. Считаются одним SQL-запросом.
type OrderStats struct {
	TotalOrders    int   // всего заказов
	PaidOrders     int   // оплаченных (payment_status = paid)
	RevenueKopecks int64 // суммарная выручка по оплаченным, в копейках
	Completed      int   // generation_status = completed
	Processing     int   // queued | processing
	Failed         int   // generation_status = failed
	OrdersToday    int   // создано за последние 24 часа
	// Демо-воронка (бесплатное демо до оплаты).
	DemosReady     int // демо успешно выдано (demo_status = ready)
	DemosToday     int // демо выдано за последние 24 часа
	DemosConverted int // демо выдано И заказ оплачен (demo ready + paid)
}
