<?php
declare(strict_types=1);

header('Content-Type: application/json; charset=utf-8');
header('Cache-Control: no-store, no-cache, must-revalidate, max-age=0');
header('Pragma: no-cache');

function first_valid_ip_from_list(string $list): ?string {
  foreach (explode(',', $list) as $ip) {
    $ip = trim($ip);
    if ($ip !== '' && filter_var($ip, FILTER_VALIDATE_IP)) return $ip;
  }
  return null;
}

function get_client_ip(): string {
  // 如果你有反代/CF，优先取这些
  if (!empty($_SERVER['HTTP_CF_CONNECTING_IP']) && filter_var($_SERVER['HTTP_CF_CONNECTING_IP'], FILTER_VALIDATE_IP)) {
    return $_SERVER['HTTP_CF_CONNECTING_IP'];
  }
  if (!empty($_SERVER['HTTP_X_REAL_IP']) && filter_var($_SERVER['HTTP_X_REAL_IP'], FILTER_VALIDATE_IP)) {
    return $_SERVER['HTTP_X_REAL_IP'];
  }
  if (!empty($_SERVER['HTTP_X_FORWARDED_FOR'])) {
    $ip = first_valid_ip_from_list($_SERVER['HTTP_X_FORWARDED_FOR']);
    if ($ip) return $ip;
  }
  return $_SERVER['REMOTE_ADDR'] ?? '0.0.0.0';
}

function http_get_json(string $url, int $timeoutSec = 2): ?array {
  if (function_exists('curl_init')) {
    $ch = curl_init($url);
    curl_setopt_array($ch, [
      CURLOPT_RETURNTRANSFER => true,
      CURLOPT_FOLLOWLOCATION => true,
      CURLOPT_CONNECTTIMEOUT => $timeoutSec,
      CURLOPT_TIMEOUT => $timeoutSec,
      CURLOPT_SSL_VERIFYPEER => false,
      CURLOPT_SSL_VERIFYHOST => 0,
      CURLOPT_HTTPHEADER => ['Accept: application/json'],
    ]);
    $raw = curl_exec($ch);
    $code = curl_getinfo($ch, CURLINFO_HTTP_CODE);
    curl_close($ch);
    if ($raw === false || $code < 200 || $code >= 300) return null;
    $j = json_decode($raw, true);
    return is_array($j) ? $j : null;
  }

  // 兜底：无 curl 的情况
  $ctx = stream_context_create([
    'http' => ['timeout' => $timeoutSec, 'header' => "Accept: application/json\r\n"],
    'ssl'  => ['verify_peer' => false, 'verify_peer_name' => false],
  ]);
  $raw = @file_get_contents($url, false, $ctx);
  if ($raw === false) return null;
  $j = json_decode($raw, true);
  return is_array($j) ? $j : null;
}

function isp_to_code(string $ispText): ?string {
  $t = strtolower($ispText);

  // 移动
  if (strpos($ispText, '移动') !== false || strpos($t, 'china mobile') !== false || strpos($t, 'cmcc') !== false) return 'CM';
  // 电信
  if (strpos($ispText, '电信') !== false || strpos($t, 'china telecom') !== false || strpos($t, 'ctcc') !== false || strpos($t, 'chinanet') !== false) return 'CT';
  // 联通
  if (strpos($ispText, '联通') !== false || strpos($t, 'china unicom') !== false || strpos($t, 'cucc') !== false) return 'CU';

  return null;
}

$ip = get_client_ip();

// 依次尝试多个免费 GEO/ISP 服务（任意一个成功即可）
$ispText = '';

$providers = [
  // ipwho.is: 结构稳定，通常能拿到 connection.isp
  [
    'url' => "https://ipwho.is/" . urlencode($ip),
    'extract' => function(array $j): string {
      return (string)($j['connection']['isp'] ?? $j['connection']['org'] ?? '');
    }
  ],
  // ip-api.com: 能返回 isp/org（http 免费，某些环境可能访问不通）
  [
    'url' => "http://ip-api.com/json/" . urlencode($ip) . "?fields=status,isp,org&lang=zh-CN",
    'extract' => function(array $j): string {
      if (($j['status'] ?? '') !== 'success') return '';
      return (string)($j['isp'] ?? $j['org'] ?? '');
    }
  ],
  // ipapi.co: org 里一般也带运营商信息（有频率限制）
  [
    'url' => "https://ipapi.co/" . urlencode($ip) . "/json/",
    'extract' => function(array $j): string {
      return (string)($j['org'] ?? '');
    }
  ],
];

foreach ($providers as $p) {
  $j = http_get_json($p['url'], 2);
  if (!$j) continue;
  $tmp = trim(($p['extract'])($j));
  if ($tmp !== '') { $ispText = $tmp; break; }
}

$code = $ispText ? isp_to_code($ispText) : null;

echo json_encode([
  'ip' => $ip,
  'isp' => $code,
  'isp_text' => $ispText !== '' ? $ispText : null,
], JSON_UNESCAPED_UNICODE);
