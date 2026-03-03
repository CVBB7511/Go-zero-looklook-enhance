# ==========================================
# 微信支付回调网关压测脚本 (Windows 11 修正版)
# ==========================================

$HostPort = "127.0.0.1:1002"
$Route = "/payment/v1/thirdPayment/thirdPaymentWxPayCallback" 
$TargetUrl = "http://${HostPort}${Route}"

# 获取当前目录下 json 文件的绝对路径，防止 hey 找不到文件
$PayloadFile = (Resolve-Path ".\wxpay_mock.json").Path
$ContentType = "application/json"

$TotalRequests = 20000  # 降低总数，本地压测 2 万次足以看出 GC 差异
$Concurrency = 50       # 并发降到 50，防止本地连接池爆满导致 Timeout
$Timeout = 60           # 增加单次请求超时时间到 60 秒

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "目标 URL : $TargetUrl" -ForegroundColor Cyan
Write-Host "并发连接 : $Concurrency" -ForegroundColor Cyan
Write-Host "总请求数 : $TotalRequests" -ForegroundColor Cyan
Write-Host "超时时间 : $Timeout 秒" -ForegroundColor Cyan
Write-Host "提示: 出现 400 状态码是正常的，因为未通过微信验签，但内存池已被触发！" -ForegroundColor Yellow
Write-Host "==========================================" -ForegroundColor Cyan

# 执行压测 (新增 -t 参数)
hey.exe -n $TotalRequests -c $Concurrency -t $Timeout -m POST -T $ContentType -D $PayloadFile $TargetUrl

Write-Host "压测结束！" -ForegroundColor Green