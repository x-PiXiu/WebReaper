# 【电商一键成片API】使用全流程说明

## 创建成片任务

### 接口地址

```
POST https://api.vidu.cn/ent/v2/ad-one-click
```

### 请求头

| 字段 | 值 | 描述 |
| --- | --- | --- |
| Content-Type | application/json | 数据交换格式 |
| Authorization | Token {your api key} | 将 {your api key} 替换为您的 token |

### 请求体

| 字段 | 必传 | 类型 | 说明 |
| --- | --- | --- | --- |
| images | 是 | Array\[String\] | 需要生成广告成片的商品图、模特图（可选）等<br>注1：支持传入图片 Base64 编码或图片URL（确保可访问）；<br>注2：支持输入 1\~7 张图；<br>注3：图片支持 png、jpeg、jpg、webp格式；<br>注4：图片比例需要小于 1:4 或者 4:1 ；<br>注5：图片大小不超过 50 MB；<br>注6：请注意，http请求的post body不超过 **20MB**，且编码必须包含适当的内容类型字符串，例如：<br>data:image/png;base64,{base64_encode}<br>注7：首张商品图为正面展示的效果最好 |
| prompt | 可选 | string | 用户提示词<br>生成视频的文本描述。<br>注1：字符长度不能超过 2000 个字符 |
| duration | 可选 | int | 时长，8～60s，默认为15s |
| aspect_ratio | 可选 | string | 输出比例，默认16:9<br>可选值 1:1，16:9，9:16 |
| language | 可选 | string | 台词或旁白使用的语言，默认zh<br>可选值 zh，en |
| creative | 可选 | bool | 成片中是否需要创意片段，默认为false<br>- false：真实成片<br>- true：创意成片 |
| callback_url | 可选 | string | Callback 协议<br>需要您在创建任务时主动设置 callback_url，请求方法为 POST，当视频生成任务有状态变化时，Vidu 将向此地址发送包含任务最新状态的回调请求。回调请求内容结构与查询任务API的返回体一致<br>回调返回的"status"包括以下状态：<br>- processing 任务处理中<br>- success 任务完成（如发送失败，回调三次）<br>- failed 任务失败（如发送失败，回调三次）<br>Vidu采用回调签名算法进行认证，详情见：[回调签名算法](https://platform.vidu.cn/docs/callback-signature) |

```
curl -X POST -H "Authorization: Token {your_api_key}" -H "Content-Type: application/json" -d '
{
    "images": ["https://prod-ss-images.s3.cn-northwest-1.amazonaws.com.cn/vidu-maas/template/image2video.png"],
    "prompt": "",
    "duration": 15,
    "aspect_ratio": "16:9",
    "language": "zh"
}' https://api.vidu.cn/ent/v2/ad-one-click
```

### 响应体

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| task_id | string | 本次任务vidu生成的任务id |
| state | string | 任务状态<br>可选值：<br>created创建成功<br>queueing任务排队中<br>processing任务处理中<br>success任务成功<br>failed任务失败 |
| images | Array\[String\] | 本次成片中使用的图片 |
| prompt | string | 本次成片中用户输入的提示词 |
| duration | int | 本次成片生成的视频时长 |
| aspect_ratio | string | 本次成片的视频比例 |
| language | string | 本次成片中旁白/台词生成的语言 |
| credits | int | 本次成片消耗的积分点数 |

```
{
  "task_id": "your_task_id_here",
  "state": "created",
  "images": ["https://prod-ss-images.s3.cn-northwest-1.amazonaws.com.cn/vidu-maas/template/image2video.png"],
  "prompt": "",
  "duration": 15,
  "aspect_ratio": "16:9",
  "language": "zh",
  "credits":credits_number
}
```

## 查询任务接口（成片、单个分镜）

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
| id | String | 是 | 成片id或子任务id |

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

## 查询子任务列表

### 说明

用于查询成片任务的所有子任务，包含：

- 分镜列表，以及每个分镜下的视频信息集合
- 旁白任务信息集合
- 背景音乐任务信息集合
- 合成成片任务信息集合

### 接口地址：

```
GET https://api.vidu.cn/ent/v2/ad-one-click/{id}
```

### 请求头

| 字段 | 值 | 描述 |
| --- | --- | --- |
| Content-Type | application/json | 数据交换格式 |
| Authorization | Token {your api key} | 将 {your api key} 替换为您的 token |

### 请求体

| 参数名称 | 类型 | 必填 | 参数描述 |
| --- | --- | --- | --- |
| id | String | 是 | 成片任务id，由创建任务接口创建成功返回 |

```
curl -X GET -H "Authorization: Token {your_api_key}" https://api.vidu.cn/ent/v2/ad-one-click/{id}
```

### 响应体

| 字段 | 类型 | 描述 |
| --- | --- | --- |
| id | string | 本次成片任务id |
| err_code | string | 错误码 |
| state | string | 任务状态<br>created创建成功<br>queueing任务排队中<br>processing任务处理中<br>success任务成功<br>failed任务失败 |
| data_records | object | 子任务集合 |

**data_records**

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| storyboards | Array of storyboard | 分镜任务记录 |
| narration_records | Array of record | 旁白任务记录 |
| bgm_records | Array of record | 背景音乐任务记录 |
| completed_creation_records | Array of composed_task | 合成成片任务记录 |

**storyboard**

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| stroyboard_id | int | 分镜序号，从0开始，例如 0，1，2，3 |
| records | Array | 当前分镜序号的所有视频任务记录 |

**record**

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| id | string | 子任务唯一id |
| state | string | 任务状态<br>created创建成功<br>queueing任务排队中<br>processing任务处理中<br>success任务成功<br>failed任务失败 |
| err_code | string | 错误码 |
| type | string | 任务类型<br>generate_video 生视频任务<br>generate_narration 生成旁白任务<br>generate_bgm 生成背景音乐任务 |
| prompt | string | 根据任务类型不同，有不同的含义：<br>generate_video 用于生成分镜的提示词<br>generate_narration 旁白内容<br>generate_bgm 用于生成背景音乐的提示词 |
| creation_url | string | 生成物url |

**composed_task**

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| id | string | 子任务唯一id |
| state | string | 任务状态<br>created创建成功<br>queueing任务排队中<br>processing任务处理中<br>success任务成功<br>failed任务失败 |
| err_code | string | 错误码 |
| creation_url | string | 生成物url |
| params | object | 参数信息 |
| params.video_task_ids | array of string | 参与合成的视频任务id列表 |
| params.bgm_task_id | string | 参与合成的bgm任务id |
| params.bgm_task_id | string | 参与合成的旁白任务id |

```
{
    "id": "891100896237203456",
    "err_code": "",
    "state": "success",
    "data_records": {
        "storyboards": [
            {
                "stroyboard_id": 0,
                "records": [
                    {
                        "id": "891101095923822592",
                        "type": "generate_video",
                        "prompt": "string",
                        "creation_url": "string",
                        "state": "success",
                        "err_code": ""
                    },
                    {
                        "id": "891101095923822593",
                        "type": "generate_video",
                        "prompt": "string",
                        "creation_url": "string",
                        "state": "success",
                        "err_code": ""
                    },
                ]
            },
            {
                "stroyboard_id": 1,
                "records": [
                    {
                        "id": "891101095923822594",
                        "type": "generate_video",
                        "prompt": "string",
                        "creation_url": "string",
                        "state": "success",
                        "err_code": ""
                    }
                ]
            },
            {
                "stroyboard_id": 2,
                "records": [
                    {
                        "id": "891101095923822595",
                        "type": "generate_video",
                        "prompt": "string",
                        "creation_url": "string",
                        "state": "success",
                        "err_code": ""
                    }
                 ]
            }
        ],
        "narration_records": [
            {
                "id": "891101095923822596",
                "type": "generate_video",
                "prompt": "string",
                "creation_url": "string",
                "state": "success",
                "err_code": ""
            }
        ],
        "bgm_records": [
            {
                "id": "891101095923822597",
                "type": "generate_video",
                "prompt": "string",
                "creation_url": "string",
                "state": "success",
                "err_code": ""
            }
        ],
        "completed_creation_records": [
            {
                "id": "891100914109132807",
                "state": "success",
                "params": {
                    "video_task_ids": [
                        "891101095923822592",
                        "891101095923822593",
                        "891101095923822594",
                        "891101095923822595",
                    ],
                    "bgm_task_id": "891101095923822597",
                    "narration_task_id": "891101095923822596"
                },
                "creation_url": "string"
                "err_code": ""
            }
        ]
    }
}
```

## 分镜编辑接口

### 接口地址：

```
POST https://api.vidu.cn/ent/v2/ad-one-click/edit
```

### 请求头

| 字段 | 值 | 描述 |
| --- | --- | --- |
| Content-Type | application/json | 数据交换格式 |
| Authorization | Token {your api key} | 将 {your api key} 替换为您的 token |

### 请求体

| 字段 | 必传 | 类型 | 说明 |
| --- | --- | --- | --- |
| ad_one_click_task_id | 是 | string | 一键成片主任务id |
| type | 是 | string | 任务类型<br>generate_video 生视频任务<br>generate_narration 生成旁白任务<br>generate_bgm 生成背景音乐任务 |
| storyboard_video_index | 可选 | int | 分镜序号，type为generate_video时必传，用于指定想要修改的分镜编号<br>注意：分镜序号从0起始 |
| prompt | 是 | string | 根据任务类型不同，有不同的含义：<br>generate_video 用于生成分镜的提示词<br>generate_narration 用于生成成片旁白的文本（音色不变）<br>generate_bgm 用于生成成片背景音乐的提示词<br>注：修改时，时长不变 |
| callback_url | 可选 | string | Callback 协议<br>需要您在创建任务时主动设置 callback_url，请求方法为 POST，当视频生成任务有状态变化时，Vidu 将向此地址发送包含任务最新状态的回调请求。回调请求内容结构与查询任务API的返回体一致<br>回调返回的"status"包括以下状态：<br>- processing 任务处理中<br>- success 任务完成（如发送失败，回调三次）<br>- failed 任务失败（如发送失败，回调三次）<br>Vidu采用回调签名算法进行认证，详情见：\~[回调签名算法](https://platform.vidu.cn/docs/callback-signature)\~ |
| payload | 可选 | string | Callback 透传参数 |

**编辑分镜**

```
curl -X POST -H "Authorization: Token {your_api_key}" -H "Content-Type: application/json" -d '
{
    "ad_one_click_task_id": "123",
    "type": "generate_video",
    "soryboard_video_index": 2,
    "prompt": "your_prompt"
}' https://api.vidu.cn/ent/v2/ad-one-click/edit
```

**编辑旁白**

```
curl -X POST -H "Authorization: Token {your_api_key}" -H "Content-Type: application/json" -d '
{
    "ad_one_click_task_id": "123",
    "type": "generate_narration"
    "prompt": "your_prompt"
}' https://api.vidu.cn/ent/v2/ad-one-click/edit
```

**编辑背景音乐**

```
curl -X POST -H "Authorization: Token {your_api_key}" -H "Content-Type: application/json" -d '
{
    "ad_one_click_task_id": "123",
    "type": "generate_bgm",
    "prompt": "your_prompt"
}' https://api.vidu.cn/ent/v2/ad-one-click/edit
```

### 响应体

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| ad_one_click_task_id | string | 一键成片主任务id |
| sub_task_id | string | 本次编辑的任务id，可在查询生成物接口中获取任务状态 |
| state | string | 任务状态<br>created创建成功<br>queueing任务排队中<br>processing任务处理中<br>success任务成功<br>failed任务失败 |
| credits | int | 本次成片消耗的积分点数 |

```
{
  "ad_one_click_task_id":"your_ad_one_click_task_id",
  "sub_task_id": "new_sub_task_id",
  "state": "success",
  "err_code": "",
  "credits": 400
}
```

## 分镜合成接口

### 接口地址：

```
POST https://api.vidu.cn/ent/v2/ad-one-click/compose
```

### 请求头

| 字段 | 值 | 描述 |
| --- | --- | --- |
| Content-Type | application/json | 数据交换格式 |
| Authorization | Token {your api key} | 将 {your api key} 替换为您的 token |

### 请求体

| 字段 | 必传 | 类型 | 说明 |
| --- | --- | --- | --- |
| ad_one_click_task_id | 是 | string | 一键成片主任务id（通过创建任务接口获取） |
| video_task_ids | 是 | Array of string | 视频任务id列表，必须与原主任务的分镜数量相同 |
| bgm_task_id | 是 | string | 背景音乐任务id |
| narration_task_id | 是 | string | 旁白任务id |
| callback_url | 可选 | string | Callback 协议<br>需要您在创建任务时主动设置 callback_url，请求方法为 POST，当视频生成任务有状态变化时，Vidu 将向此地址发送包含任务最新状态的回调请求。回调请求内容结构与查询任务API的返回体一致<br>回调返回的"status"包括以下状态：<br>- processing 任务处理中<br>- success 任务完成（如发送失败，回调三次）<br>- failed 任务失败（如发送失败，回调三次）<br>Vidu采用回调签名算法进行认证，详情见：\~[回调签名算法](https://platform.vidu.cn/docs/callback-signature)\~ |
| payload | 可选 | string | Callback 透传参数 |

```
curl -X GET -H "Authorization: Token {your_api_key}" -H "Content-Type: application/json" -d '
{
    "ad_one_click_task_id": "123",
    "video_task_ids": ["your_sub_task_id_1","your_sub_task_id_2","your_sub_task_id_3"],
    "bgm_task_id": "your_bgm_id_1",
    "narration_task_id":"your_narration_id_1"
}' https://api.vidu.cn/ent/v2/ad-one-click/compose
```

### 响应体

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| ad_one_click_task_id | string | 一键成片主任务id |
| compose_sub_task_id | string | 本次重新合成的任务id, 可在查询生成物接口中获取任务状态 |
| state | string | 任务状态<br>created创建成功<br>queueing任务排队中<br>processing任务处理中<br>success任务成功<br>failed任务失败 |
| credits | int | 本次成片消耗的积分点数 |

```
{
  "ad_one_click_task_id":"your_task_id",
  "compose_sub_task_id": "new_id",
  "state": "success",
  "credits": 400
}
```