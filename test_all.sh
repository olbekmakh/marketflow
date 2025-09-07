#!/bin/bash

BASE_URL="http://localhost:8080"
SYMBOLS=("BTCUSDT" "ETHUSDT" "DOGEUSDT" "TONUSDT" "SOLUSDT")

echo "🧪 Testing MarketFlow API..."
echo "================================"

# Test health
echo "1. Testing health endpoint:"
curl -s "$BASE_URL/health" | jq .data.status
echo ""

# Test mode switching
echo "2. Testing mode switching:"
echo "  Switching to test mode:"
curl -s -X POST "$BASE_URL/mode/test" | jq .data.mode
echo "  Switching to live mode:"
curl -s -X POST "$BASE_URL/mode/live" | jq .data.mode
echo ""

# Test latest prices
echo "3. Testing latest prices:"
for symbol in "${SYMBOLS[@]}"; do
    echo "  $symbol:"
    curl -s "$BASE_URL/prices/latest/$symbol" | jq '.data | {symbol, price}'
done
echo ""

# Test highest prices
echo "4. Testing highest prices:"
for symbol in "${SYMBOLS[@]}"; do
    echo "  $symbol (5m period):"
    curl -s "$BASE_URL/prices/highest/$symbol?period=5m" | jq '.data | {pair_name, max_price, period}'
done
echo ""

# Test lowest prices
echo "5. Testing lowest prices:"
for symbol in "${SYMBOLS[@]}"; do
    echo "  $symbol (1h period):"
    curl -s "$BASE_URL/prices/lowest/$symbol?period=1h" | jq '.data | {pair_name, min_price, period}'
done
echo ""

# Test average prices
echo "6. Testing average prices:"
for symbol in "${SYMBOLS[@]}"; do
    echo "  $symbol (30s period):"
    curl -s "$BASE_URL/prices/average/$symbol?period=30s" | jq '.data | {pair_name, average_price, period}'
done

echo ""
echo "✅ All tests completed!"
