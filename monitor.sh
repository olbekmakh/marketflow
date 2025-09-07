#!/bin/bash
while true; do
    echo "=== $(date) ==="
    echo "Health:"
    curl -s http://localhost:8080/health | jq .data.status
    
    echo "BTC Price:"
    curl -s http://localhost:8080/prices/latest/BTCUSDT | jq .data.price
    
    echo "ETH Price:"  
    curl -s http://localhost:8080/prices/latest/ETHUSDT | jq .data.price
    
    echo "---"
    sleep 5
done
