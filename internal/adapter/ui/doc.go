// Package ui is the primary (driving) adapter for electrorangerd.
// It owns the GIOUI desktop window, translates user interactions into calls
// on the inbound port interfaces, and renders the ERD canvas. It contains no
// business logic — all domain decisions flow through the port layer.
package ui
