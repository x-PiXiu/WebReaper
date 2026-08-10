# ZPAY 支付接口文档

> 来源：https://zpayz.cn  
> 签名算法：MD5  
> 编码：UTF-8

---

## 1. 页面跳转支付

适用于用户前台直接发起支付，使用 form 表单跳转或拼接 URL 跳转。

- **请求 URL**：`https://zpayz.cn/submit.php`
- **请求方法**：POST（推荐）或 GET

### 请求参数

| 参数 | 名称 | 类型 | 必填 | 描述 | 示例 |
|------|------|------|------|------|------|
| `name` | 商品名称 | String | 是 | 需体现出具体售卖的商品，否则容易被封 | iPhone17苹果手机 |
| `money` | 订单金额 | String | 是 | 最多保留两位小数 | 5.67 |
| `type` | 支付方式 | String | 是 | `alipay`（支付宝）/ `wxpay`（微信） | alipay |
| `out_trade_no` | 商户订单号 | String | 是 | 每个商品不可重复，最多 32 位 | 201911914837526544601 |
| `notify_url` | 异步通知页面 | String | 是 | 交易信息回调页面，不支持带参数 | http://www.aaa.com/bbb.php |
| `pid` | 商户唯一标识 | String | 是 | 一串字母数字组合 | 201901151314084206659771 |
| `cid` | 支付渠道 ID | String | 否 | 支持填写多个，使用 `,` 隔开，不填则随机调用 | 1234 |
| `param` | 附加内容 | String | 否 | 会通过 notify_url 原样返回 | 金色 256G |
| `return_url` | 跳转页面 | String | 是 | 交易完成后浏览器跳转，不支持带参数 | http://www.aaa.com/ccc.php |
| `sign` | 签名 | String | 是 | 采用 MD5 加密 | 28f9583617d9caf66834292b6ab1cc89 |
| `sign_type` | 签名方法 | String | 是 | 默认为 MD5 | MD5 |

### 成功返回

直接跳转到付款页面（收银台）。

### 失败返回

```json
{ "code": "error", "msg": "具体的错误信息" }
```

---

## 2. API 接口支付

适用于后端发起支付请求，返回二维码或支付链接。

- **请求 URL**：`https://zpayz.cn/mapi.php`
- **请求方法**：POST（form-data）

### 请求参数

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
|--------|--------|------|------|--------|------|
| 商户 ID | `pid` | 是 | String | 1001 | |
| 支付渠道 ID | `cid` | 否 | String | 1234 | 支持多个，逗号隔开，不填随机 |
| 支付方式 | `type` | 是 | String | alipay | `alipay` / `wxpay` |
| 商户订单号 | `out_trade_no` | 是 | String | 20160806151343349 | 不可重复，最多 32 位 |
| 异步通知地址 | `notify_url` | 是 | String | http://pay.com/notify.php | 服务器异步通知地址 |
| 商品名称 | `name` | 是 | String | iPhone17苹果手机 | 体现具体商品，否则易被封 |
| 商品金额 | `money` | 是 | String | 1.00 | 单位：元，最大 2 位小数 |
| 用户 IP 地址 | `clientip` | 是 | String | 192.168.1.100 | 用户发起支付的 IP |
| 设备类型 | `device` | 否 | String | pc | 浏览器 UA 类型，默认 pc |
| 业务扩展参数 | `param` | 否 | String | | 支付后原样返回 |
| 签名字符串 | `sign` | 是 | String | 202cb962ac59075b964b07152d234b70 | MD5 签名 |
| 签名类型 | `sign_type` | 是 | String | MD5 | 默认 MD5 |

### 成功返回

| 字段名 | 变量名 | 类型 | 示例值 | 描述 |
|--------|--------|------|--------|------|
| 返回状态码 | `code` | Int | 1 | 1 为成功，其它为失败 |
| 返回信息 | `msg` | String | | 失败时返回原因 |
| 订单号 | `trade_no` | String | 20160806151343349 | 支付订单号 |
| ZPAY 内部订单号 | `O_id` | String | 123456 | |
| 支付跳转 url | `payurl` | String | https://xxx.cn/pay/wxpay/... | 直接跳转支付 |
| 二维码链接 | `qrcode` | String | https://xxx.cn/pay/wxpay/... | 根据该 URL 生成二维码 |
| 二维码图片 | `img` | String | https://zpayz.cn/qrcode/123.jpg | 付款二维码图片地址 |

### 失败返回

```json
{ "code": "error", "msg": "具体的错误信息" }
```

---

## 3. 查询单个订单

- **请求 URL**：`https://zpayz.cn/api.php?act=order&pid={商户ID}&key={商户密钥}&out_trade_no={商户订单号}`
- **请求方法**：GET

### 请求参数

| 参数 | 名称 | 类型 | 必填 | 描述 |
|------|------|------|------|------|
| `act` | 操作类型 | String | 是 | 固定值 `order` |
| `pid` | 商户 ID | String | 是 | |
| `key` | 商户密钥 | String | 是 | |
| `trade_no` | 系统订单号 | String | 二选一 | |
| `out_trade_no` | 商户订单号 | String | 二选一 | |

### 返回结果

| 字段名 | 变量名 | 类型 | 示例值 | 描述 |
|--------|--------|------|--------|------|
| 返回状态码 | `code` | Int | 1 | 1 为成功 |
| 返回信息 | `msg` | String | 查询订单号成功！ | |
| 易支付订单号 | `trade_no` | String | | |
| 商户订单号 | `out_trade_no` | String | | |
| 支付方式 | `type` | String | alipay | |
| 商户 ID | `pid` | String | | |
| 创建时间 | `addtime` | String | 2016-08-06 22:55:52 | |
| 完成时间 | `endtime` | String | | |
| 商品名称 | `name` | String | VIP 会员 | |
| 商品金额 | `money` | String | 1.00 | |
| 支付状态 | `status` | Int | 0 | 1=成功，0=未支付 |
| 业务扩展参数 | `param` | String | | |
| 支付者账号 | `buyer` | String | | |

---

## 4. 提交订单退款

- **请求 URL**：`https://zpayz.cn/api.php?act=refund`
- **请求方法**：POST

### 请求参数

| 字段名 | 变量名 | 必填 | 类型 | 描述 |
|--------|--------|------|------|------|
| 商户 ID | `pid` | 是 | String | |
| 商户密钥 | `key` | 是 | String | |
| 易支付订单号 | `trade_no` | 二选一 | String | |
| 商户订单号 | `out_trade_no` | 二选一 | String | |
| 退款金额 | `money` | 是 | String | 大多数通道需与原订单金额一致 |

### 返回结果

| 字段名 | 变量名 | 类型 | 描述 |
|--------|--------|------|------|
| 返回状态码 | `code` | Int | 1 为成功 |
| 返回信息 | `msg` | String | |

---

## 5. 支付结果通知

异步回调通知，通过 `notify_url` 和 `return_url` 接收。

- **请求方法**：GET

### 回调参数

| 参数 | 名称 | 类型 | 描述 | 示例 |
|------|------|------|------|------|
| `pid` | 商户 ID | String | | 201901151314084206659771 |
| `name` | 商品名称 | String | 不超过 100 字 | iphone |
| `money` | 订单金额 | String | 最多两位小数 | 5.67 |
| `out_trade_no` | 商户订单号 | String | | |
| `trade_no` | 易支付订单号 | String | | |
| `param` | 业务扩展参数 | String | 原样返回 | 金色 256G |
| `trade_status` | 支付状态 | String | 只有 `TRADE_SUCCESS` 是成功 | TRADE_SUCCESS |
| `type` | 支付方式 | String | | alipay |
| `sign` | 签名 | String | MD5 | |
| `sign_type` | 签名类型 | String | MD5 | |

### 回调处理要求

1. 收到回调后必须返回纯字符串 `success`，否则视为通知失败
2. 同一通知可能多次发送，必须能正确处理重复通知
3. 处理前先检查业务数据状态，判断是否已处理过
4. **必须做签名验证**，并校验返回金额与商户侧金额一致，防止"假通知"
5. 未返回 success 时，平台按以下频率重试：0/15/15/30/180/1800/1800/1800/1800/3600 秒

---

## 6. MD5 签名算法

1. 将所有参数按参数名 ASCII 码从小到大排序（a-z），`sign`、`sign_type` 和空值**不参与签名**
2. 将排序后的参数拼接成 URL 键值对格式：`a=b&c=d&e=f`（参数值不做 URL 编码）
3. 将拼接好的字符串与商户密钥 KEY 拼接后做 MD5 加密：`sign = md5(a=b&c=d&e=f + KEY)`
4. MD5 结果为**小写**

### 签名示例（Java）

```java
public static String sign(Map<String, String> params, String key) {
    // 1. 过滤空值和 sign/sign_type
    TreeMap<String, String> sorted = new TreeMap<>();
    for (Map.Entry<String, String> e : params.entrySet()) {
        if (e.getValue() != null && !e.getValue().isEmpty()
                && !e.getKey().equals("sign")
                && !e.getKey().equals("sign_type")) {
            sorted.put(e.getKey(), e.getValue());
        }
    }
    // 2. 拼接为 URL 键值对
    StringBuilder sb = new StringBuilder();
    for (Map.Entry<String, String> e : sorted.entrySet()) {
        if (sb.length() > 0) sb.append("&");
        sb.append(e.getKey()).append("=").append(e.getValue());
    }
    // 3. 追加密钥后 MD5
    sb.append(key);
    return md5(sb.toString()).toLowerCase();
}
```

---

## 7. 集成注意事项

- **商户 ID（pid）** 和 **商户密钥（key）** 从 ZPAY 后台获取
- `out_trade_no` 必须保证全局唯一，建议使用 UUID 或时间戳+随机数
- `notify_url` 和 `return_url` 不支持带参数
- `name` 商品名称需体现具体商品，过于模糊（如"充值"、"付款"）容易被封
- 金额单位为**元**，最多两位小数
