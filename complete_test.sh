#!/bin/bash

echo "🧪 Complete MarketFlow API Test"
echo "==============================="

BASE="http://localhost:8080"

echo "1. 🔧 Health Check:"
curl -s "$BASE/health" | jq '.data.status'

echo -e "\n2. 🔄 Mode Switching:"
echo "  Test mode:"
curl -s -X POST "$BASE/mode/test" | jq '.data | {mode, status}'
echo "  Live mode:"
curl -s -X POST "$BASE/mode/live" | jq '.data | {mode, status}'

echo -e "\n3. 💰 Latest Prices:"
for symbol in BTCUSDT ETHUSDT DOGEUSDT TONUSDT SOLUSDT; do
    echo "  $symbol:"
    curl -s "$BASE/prices/latest/$symbol" | jq '.data | {symbol, price}'
done

echo -e "\n4. 📈 Highest Prices (5m):"
for symbol in BTCUSDT ETHUSDT; do
    echo "  $symbol:"
    curl -s "$BASE/prices/highest/$symbol?period=5m" | jq '.data | {pair_name, max_price, period}'
done

echo -e "\n5. 📉 Lowest Prices (1h):"
for symbol in DOGEUSDT TONUSDT; do
    echo "  $symbol:"
    curl -s "$BASE/prices/lowest/$symbol?period=1h" | jq '.data | {pair_name, min_price, period}'
done

echo -e "\n6. 📊 Average Prices (30s):"
curl -s "$BASE/prices/average/SOLUSDT?period=30s" | jq '.data | {pair_name, average_price, period}'

echo -e "\n✅ All tests completed!"
