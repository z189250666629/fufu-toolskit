<?php
declare(strict_types=1);

header('Content-Type: application/json; charset=utf-8');
header('Cache-Control: no-store, no-cache, must-revalidate, max-age=0');
header('Pragma: no-cache');

// 获取真实客户端 IP
function getClientIP(): string {
    if (!empty($_SERVER['HTTP_CF_CONNECTING_IP']) && filter_var($_SERVER['HTTP_CF_CONNECTING_IP'], FILTER_VALIDATE_IP)) {
        return $_SERVER['HTTP_CF_CONNECTING_IP'];
    } elseif (!empty($_SERVER['HTTP_X_REAL_IP']) && filter_var($_SERVER['HTTP_X_REAL_IP'], FILTER_VALIDATE_IP)) {
        return $_SERVER['HTTP_X_REAL_IP'];
    } elseif (!empty($_SERVER['HTTP_X_FORWARDED_FOR'])) {
        $ipList = explode(',', $_SERVER['HTTP_X_FORWARDED_FOR']);
        foreach ($ipList as $candidate) {
            $candidate = trim($candidate);
            if (filter_var($candidate, FILTER_VALIDATE_IP)) return $candidate;
        }
    } elseif (!empty($_SERVER['REMOTE_ADDR']) && filter_var($_SERVER['REMOTE_ADDR'], FILTER_VALIDATE_IP)) {
        return $_SERVER['REMOTE_ADDR'];
    }
    return '127.0.0.1';
}

function httpGet(string $url, int $timeoutSec = 5): ?string {
    if (function_exists('curl_init')) {
        $ch = curl_init($url);
        curl_setopt_array($ch, [
            CURLOPT_RETURNTRANSFER => true,
            CURLOPT_FOLLOWLOCATION => true,
            CURLOPT_CONNECTTIMEOUT => $timeoutSec,
            CURLOPT_TIMEOUT => $timeoutSec,
            CURLOPT_USERAGENT => 'Mozilla/5.0',
        ]);
        $body = curl_exec($ch);
        $code = curl_getinfo($ch, CURLINFO_HTTP_CODE);
        curl_close($ch);
        return ($body !== false && $code >= 200 && $code < 300) ? (string)$body : null;
    }

    $ctx = stream_context_create([
        'http' => [
            'timeout' => $timeoutSec,
            'header' => "User-Agent: Mozilla/5.0\r\n",
        ],
        'ssl' => [
            'verify_peer' => false,
            'verify_peer_name' => false,
        ],
    ]);
    $body = @file_get_contents($url, false, $ctx);
    return $body === false ? null : (string)$body;
}

function httpGetJson(string $url, int $timeoutSec = 3): ?array {
    $body = httpGet($url, $timeoutSec);
    if ($body === null) return null;
    $data = json_decode($body, true);
    return is_array($data) ? $data : null;
}

$ip = getClientIP();
$asn = '';
$isp = '';
$location = '';

// 优先保留原来的 ip138 解析逻辑。
$html = httpGet("https://www.ip138.com/iplookup.php?ip=" . urlencode($ip) . "&action=2", 5);
if ($html !== null) {
    if (preg_match('/ASN归属地<\/td>\s*<td>\s*<span>([^<]+)<\/span>/i', $html, $m1)) {
        $asn = trim(html_entity_decode($m1[1], ENT_QUOTES | ENT_HTML5, 'UTF-8'));
    }
    if (preg_match('/运营商<\/td>\s*<td>([^<]+)<\/td>/i', $html, $m2)) {
        $isp = trim(html_entity_decode($m2[1], ENT_QUOTES | ENT_HTML5, 'UTF-8'));
    }
}

if ($asn !== '' || $isp !== '') {
    $location = trim($asn . ' ' . $isp);
}

// ip138 失败时兜底到 JSON 接口。
if ($location === '' || $isp === '') {
    $fallback = httpGetJson("https://ipwho.is/" . urlencode($ip), 3);
    if ($fallback && ($fallback['success'] ?? true)) {
        $country = (string)($fallback['country'] ?? '');
        $region = (string)($fallback['region'] ?? '');
        $city = (string)($fallback['city'] ?? '');
        $isp = $isp ?: (string)($fallback['connection']['isp'] ?? $fallback['connection']['org'] ?? '');
        $location = $location ?: trim($country . ' ' . $region . ' ' . $city . ' ' . $isp);
    }
}

echo json_encode([
    'ip' => $ip,
    'location' => $location !== '' ? $location : null,
    'asn' => $asn !== '' ? $asn : null,
    'isp' => $isp !== '' ? $isp : null,
], JSON_UNESCAPED_UNICODE);
