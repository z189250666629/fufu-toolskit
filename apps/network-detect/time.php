<?php
header('Content-Type: text/plain; charset=utf-8');
header('Cache-Control: no-store, no-cache, must-revalidate, max-age=0');
date_default_timezone_set("Asia/Shanghai");
echo time() * 1000; // 返回北京时间的毫秒时间戳
