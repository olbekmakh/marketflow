#!/bin/bash

echo "📊 MarketFlow Price Monitor"
echo "Press Ctrl+C to stop"
echo "=========================="

while true; do
    clear
    echo "📊 MarketFlow - $(date)"
    echo "=========================="
    
    # System health
    echo "🔧 System Status:"
    curl -s http://localhost:8080/health | jq -r '.data.status'
    echo ""
    
    # Live prices
    echo "💰 Live Prices:"
    echo "Symbol        Price         Change"
    echo "--------------------------------"
    
    for symbol in BTCUSDT ETHUSDT DOGEUSDT TONUSDT SOLUSDT; do
        price=$(curl -s "http://localhost:8080/prices/latest/$symbol" | jq -r '.data.price')
        printf "%-12s $%-12.8f 📊\n" "$symbol" "$price"
    done
    
    echo ""
    echo "🔄 Refreshing in 3 seconds..."
    sleep 3
done
