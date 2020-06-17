#!/bin/sh
TAILWIND_VER="1.4.6"
MOMENT_VER="2.25.3"
CHART_VER="2.9.3"
curl -sL https://unpkg.com/tailwindcss@^${TAILWIND_VER}/dist/tailwind.min.css > tailwind-${TAILWIND_VER}.min.css
curl -sL https://cdn.jsdelivr.net/npm/moment@${MOMENT_VER}/moment.min.js > moment-${MOMENT_VER}.min.js
curl -sL https://cdn.jsdelivr.net/npm/chart.js@${CHART_VER}/dist/Chart.min.js > chart-${CHART_VER}.min.js
