#!/bin/bash
cd "$(dirname "$0")"
mkdir -p data

# Kill any existing nanolytica
pkill -f "./nanolytica" 2>/dev/null || true
sleep 1

# Start server
./nanolytica &
PID=$!
sleep 2

echo "======================================"
echo "Testing Nanolytica Routes"
echo "======================================"

echo -e "\n1. Testing /health:"
curl -s http://localhost:8080/health | head -1

echo -e "\n2. Testing /admin (redirect):"
curl -s -o /dev/null -w "%{http_code} -> %{redirect_url}\n" http://localhost:8080/admin

echo -e "\n3. Testing /admin/analytics (redirect):"
curl -s -o /dev/null -w "%{http_code} -> %{redirect_url}\n" http://localhost:8080/admin/analytics

echo -e "\n4. Testing /admin/analytics/ (dashboard):"
curl -s -o /dev/null -w "HTTP Status: %{http_code}\nContent-Type: %{content_type}\n" http://localhost:8080/admin/analytics/

echo -e "\n5. Dashboard HTML title:"
curl -s http://localhost:8080/admin/analytics/ | grep -o "<title>.*</title>"

echo -e "\n======================================"
echo "Dashboard URL: http://localhost:8080/admin/analytics/"
echo "======================================"

kill $PID 2>/dev/null
