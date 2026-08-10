# 【视频复刻API】使用全流程说明

## 创建复刻任务

### 接口地址：

```
POST https://api.vidu.cn/ent/v2/trending-replicate
```

### 请求头

| 字段 | 值 | 描述 |
| --- | --- | --- |
| Content-Type | application/json | 数据交换格式 |
| Authorization | Token {your api key} | 将 {your api key} 替换为您的 token |

### 请求体

| 字段 | 必传 | 类型 | 说明 |
| --- | --- | --- | --- |
| video_url | 是 | String | 需要复刻的原视频<br>注1：支持传入 Base64 编码或视频URL（确保可访问）；<br>注2：支持输入 1 个视频；<br>注3：支持 mp4、mov格式；<br>注4：视频最少5秒，最多180秒；<br>注5：请注意，http请求的post body不超过 **20MB**，且编码必须包含适当的内容类型字符串，例如：<br>data:image/png;base64,{base64_encode} |
| images | 是 | Array\[String\] | 用户需要复刻的商品图、模特图（可选）<br>注1：支持传入图片 Base64 编码或图片URL（确保可访问）；<br>注2：支持输入 1\~7 张图；<br>注3：图片支持 png、jpeg、jpg、webp格式；<br>注4：图片比例需要小于 1:4 或者 4:1 ；<br>注5：图片大小不超过 50 MB；<br>注6：请注意，http请求的post body不超过 **20MB**，且编码必须包含适当的内容类型字符串，例如：<br>data:image/png;base64,{base64_encode} |
| prompt | 可选 | string | 用户提示词<br>生成视频的文本描述。<br>注1：字符长度不能超过 2000 个字符 |
| aspect_ratio | 可选 | string | 输出比例，默认16:9<br>可选值 1:1，16:9，9:16，4:3，3:4 |
| resolution | 可选 | String | 清晰度，默认1080p<br>可选值：540p、720p、1080p |
| remove_audio | 可选 | Bool | 是否去除原视频声音<br>true：去除原视频声音<br>false：保留原视频声音 |
| callback_url | 可选 | string | Callback 协议<br>需要您在创建任务时主动设置 callback_url，请求方法为 POST，当视频生成任务有状态变化时，Vidu 将向此地址发送包含任务最新状态的回调请求。回调请求内容结构与查询任务API的返回体一致<br>回调返回的"status"包括以下状态：<br>- processing 任务处理中<br>- success 任务完成（如发送失败，回调三次）<br>- failed 任务失败（如发送失败，回调三次）<br>Vidu采用回调签名算法进行认证，详情见：\~[回调签名算法](https://platform.vidu.cn/docs/callback-signature)\~ |

```
curl -X POST -H "Authorization: Token {your_api_key}" -H "Content-Type: application/json" -d '
{
    "video_url": "your_url",
    "images": ["https://prod-ss-images.s3.cn-northwest-1.amazonaws.com.cn/vidu-maas/template/image2video.png"],
    "prompt": "",
    "aspect_ratio": "16:9",
    "resolution": "1080p"
}' https://api.vidu.cn/ent/v2/trending-replicate
```

### 响应体

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| task_id | string | 本次任务vidu生成的任务id |
| state | string | 任务状态<br>可选值：<br>created创建成功<br>queueing任务排队中<br>processing任务处理中<br>success任务成功<br>failed任务失败 |
| images | Array\[String\] | 本次成片中使用的图片 |
| prompt | string | 本次成片中用户输入的提示词 |
| resolution | String | 本次成片的清晰度 |
| aspect_ratio | string | 本次成片的视频比例 |
| remove_audio | string | 本次成片中是否去除原声 |
| credits | int | 本次成片消耗的积分点数 |

```
{
  "task_id": "your_task_id_here",
  "state": "created",
  "images": ["https://prod-ss-images.s3.cn-northwest-1.amazonaws.com.cn/vidu-maas/template/image2video.png"],
  "prompt": "",
  "aspect_ratio": "16:9",
  "resolution": "1080p",
  "remove_audio": false,
  "credits":credits_number
}
```

## 查询复刻任务

### 接口地址：

```
GET https://api.vidu.cn/ent/v2/tasks/{id}/creations
```

### 请求头

| 字段 | 值 | 描述 |
| --- | --- | --- |
| Content-Type | application/json | 数据交换格式 |
| Authorization | Token {your api key} | 将 {your api key} 替换为您的 token |

### 请求体

| 参数名称 | 类型 | 必填 | 参数描述 |
| --- | --- | --- | --- |
| id | String | 是 | 任务ID |

```
curl -X GET -H "Authorization: Token {your_api_key}" https://api.vidu.cn/ent/v2/tasks/{your_id}/creations
```

### 响应体

| 字段 | 子字段 | 类型 | 描述 |
| --- | --- | --- | --- |
| id |  | String | 任务ID |
| state |  | String | 处理状态<br>可选值：<br>created 创建成功<br>queueing 任务排队中<br>processing 任务处理中<br>success 任务成功<br>failed 任务失败 |
| err_code |  | String | 错误码，具体见错误码表 |
| credits |  | Int | 该任务消耗的积分数量，单位：积分 |
| payload |  | String | 本次任务调用时传入的透传参数 |
| bgm |  | Bool | 本次任务调用是否使用bgm |
| off_peak |  | Bool | 本次任务调用是否使用错峰模式 |
| progress |  | Int | 任务生成进度 |
| creations |  | Array | 生成物结果 |
|  | id | String | 生成物id，用来标识不同的生成物 |
|  | url | String | 生成物URL， 一个小时有效期 |
|  | cover_url | String | 生成物封面，一个小时有效期 |
|  | watermarked_url | String | 带水印的生成物url，一小时有效期 |

```
{
  "id":"your_task_id",
  "state": "success",
  "err_code": "",
  "credits": 400,
  "payload":""
  "creations": [
    {
      "id": "your_creations_id",
      "url": "your_generated_results_url",
      "cover_url": "your_generated_results_cover_url",
      "watermarked_url": "your_generated_results_watermarked_url"
    }
  ],
  "progress":90
}
```