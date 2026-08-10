# 【一键生成MV】使用全流程

## 创建MV任务接口

### 接口地址：

```
POST https://api.vidu.cn/ent/v2/one-click/mv
```

### 请求头

| 字段 | 值 | 描述 |
| --- | --- | --- |
| Content-Type | application/json | 数据交换格式 |
| Authorization | Token {your api key} | 将 {your api key} 替换为您的 token |

### 请求体


| 字段             | 必传  | 类型            | 说明                                                           |
|----------------|-----|---------------|--------------------------------------------------------------|
| images         | 是   | Array[String] | 用户生成mv时的模特图片或风格图片<br>注1：支持传入图片 Base64 编码或图片URL（确保可访问）；<br>注2：支持输入 1~7 张图；<br>注3：图片支持 png、jpeg、jpg、webp格式；注4：图片比例需要小于 1:4 或者 4:1 ；<br>注5：图片大小不超过 50 MB；<br>注6：请注意，http请求的post body不超过 20MB，且编码必须包含适当的内容类型字符串，例如：data:image/png;base64,{base64_encode} |
| audio_url      | 是   | String        | 需要生成mv的音频<br>注1：支持传入 Base64 编码或音频URL（确保可访问）；<br>注2：支持输入 1 个音频；<br>注3：支持 mp3、wav、aac、m4a格式；<br>注4：一次输出合成完的音频，上传音频最少10秒，最多180秒；<br>注5: 仅输出分镜列表，上传音频最少10秒，最多300秒；<br>注6：base64 decode之后的字节长度需要小于20M，且编码必须包含适当的内容类型字符串，例如：data:image/png;base64,{base64_encode} |
| lip_sync       | 可选  | bool          | true：需要对口型false：不需要对口型默认false开启之后，每秒增加4积分。<br>注1：必须传入lip_ref_url字段，对口型才生效；<br>注2：不支持非真人； <br>注3：不支持视频中非正面的片段；<br>注4：540p不支持对口型 |
| lip_ref_url    | 可选  | string        | url，唱歌的人脸，需要正面对镜头，近景<br>图像文件要求：<br>- 内容：需包含一张清晰的人物正脸，且为视频中出现的人物<br>- 文件大小：文件≤10MB - 图像大小：长宽比小于等于2，最大边长小于等4096<br>- 格式：jpeg、jpg、png、bmp、webp |
| prompt         | 可选  | string        | 用户提示词生成mv视频的文本描述<br>注1：字符长度不能超过 3000 个字符                     |
| aspect_ratio   | 可选  | string        | 输出比例，默认16:9可选值 1:1，16:9，9:16，4:3，3:4                         |
| quality        | 可选  | string        | standard：标准版<br>high：高级版，画面表现、人脸一致性双重增强                      |
| resolution     | 可选  | String        | 清晰度，默认720p可选值：540p、720p、1080p                                |
| add_subtitle   | 可选  | Bool          | 是否需要字幕，默认为 falsetrue：需要字幕false：不需要字幕                         |
| subtitle_color | 可选  | string        | 默认 #FFFFFF，白色HEX格式字色                                         |
| language       | 可选  | String        | 音频语言类型，默认为自动可选 en、zh                                         |
| srt_url        | 可选  | String        | 字幕文件url                                                      |
| callback_url   | 可选  | String        | Callback 协议需要您在创建任务时主动设置 callback_url，请求方法为 POST，当视频生成任务有状态变化时，Vidu 将向此地址发送包含任务最新状态的回调请求。回调请求内容结构与查询任务API的返回体一致回调返回的"status"包括以下状态：- processing 任务处理中- success 任务完成（如发送失败，回调三次）- failed 任务失败（如发送失败，回调三次）Vidu采用回调签名算法进行认证，详情见：回调签名算法 |


```
curl -X POST -H "Authorization: Token {your_api_key}" -H "Content-Type: application/json" -d '
{
    "images": ["https://prod-ss-images.s3.cn-northwest-1.amazonaws.com.cn/vidu-maas/template/image2video.png"],
    "prompt": "",
    "audio_url":"",
    "aspect_ratio": "16:9"
}' https://api.vidu.cn/ent/v2/one-click/mv
```


### 响应体

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| id | string | 本次任务vidu生成的任务id |

```
{
  "id": "your_id_here"
}
```

## 查询任务接口

### 接口地址：

```
GET https://api.vidu.cn/ent/v2/one-click/mv/{id}
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
curl -X GET -H "Authorization: Token {your_api_key}" https://api.vidu.cn/ent/v2/one-click/mv/{id}
```

### 响应体

| 字段 | 子字段 |  | 类型 | 描述 |
| --- | --- | --- | --- | --- |
| id |  |  | String | 任务ID |
| state |  |  | String | 处理状态<br>可选值：<br>created 创建成功<br>processing 任务处理中<br>success 任务成功<br>failed 任务失败 |
| err_code |  |  | String | 错误码，具体见错误码表 |
| err_msg |  |  | String | 错误相关信息 |
| created_at |  |  | String | 创建时间 |
| signed_url |  |  | String | 成片生成物url(小于3分钟会有) |
| job_records |  |  | Array | 分镜列表+合成视频 |
|  | type |  | String | 可选值:<br>generate_video 视频分镜<br>compose 合成所有分镜 |
|  | index |  | Int | 分镜索引 |
|  | jobs |  | Array | 单个分镜修改历史 |
|  |  | id | String |  |
|  |  | type | String | generate_video/compose |
|  |  | state | String | 处理状态<br>可选值：<br>created 创建成功<br>processing 任务处理中<br>success 任务成功<br>failed 任务失败 |
|  |  | signed_url |  | 分镜或合成生成物 url |

```
{
    "id": "901243326567555072",
    "type": "mv",
    "creator_id": "869085108764676096",
    "state": "success",
    "input": "",
    "job_records": [
        {
            "type": "generate_video",
            "jobs": [
                {
                    "id": "901245331704909824",
                    "type": "generate_video",
                    "input": "",
                    "output": "",
                    "state": "success",
                    "created_at": "2025-12-24T02:59:06.937309Z",
                    "updated_at": "2025-12-24T02:59:35.473978Z",
                    "err_code": "",
                    "err_msg": "",
                    "signed_url": "singed_url",
                    "mv_generate_video_input": {
                        "duration": 3,
                        "prompt": "2.5d建模风格，明亮活泼的色调，极光在北极圈上空舞动，粒子光效闪烁，镜头缓慢推进，营造神秘又欢快的氛围。",
                        "lyric_text": "前奏",
                        "signed_images": []
                    },
                    "index": 0
                }
            ],
            "index": 0
        },
        {
            "type": "compose",
            "jobs": [
                {
                    "id": "901245503243554816",
                    "type": "compose",
                    "input": "",
                    "output": "",
                    "state": "success",
                    "created_at": "2025-12-24T02:59:47.431993Z",
                    "updated_at": "2025-12-24T03:00:13.727780Z",
                    "err_code": "",
                    "err_msg": "",
                    "signed_url": "signed_url",
                    "index": 0
                }
            ],
            "index": 0
        }
    ],
    "created_at": "2025-12-24T02:51:34.070650Z",
    "updated_at": "2025-12-24T03:00:13.734086Z",
    "err_code": "",
    "err_msg": "",
    "signed_url": "singed_url"
}
```

## 编辑任务接口

### 接口地址：

```
POST https://api.vidu.cn/ent/v2/one-click/mv/{id}/edit
```

### 请求头

| 字段 | 值 | 描述 |
| --- | --- | --- |
| Content-Type | application/json | 数据交换格式 |
| Authorization | Token {your api key} | 将 {your api key} 替换为您的 token |

### 请求体

| 字段 | 必传 | 类型 | 说明 |
| --- | --- | --- | --- |
| id | 是 | String | 任务id，放在path路径中 |
| job_id | 是 | String | 要编辑的分镜id |
| prompt | 是 | string | 用户提示词<br>生成mv视频的文本描述。<br>注1：字符长度不能超过 3000 个字符 |
| callback_url | 可选 | String | Callback 协议<br>需要您在创建任务时主动设置 callback_url，请求方法为 POST，当视频生成任务有状态变化时，Vidu 将向此地址发送包含任务最新状态的回调请求。回调请求内容结构与查询任务API的返回体一致<br>回调返回的"status"包括以下状态：<br>- processing 任务处理中<br>- success 任务完成（如发送失败，回调三次）<br>- failed 任务失败（如发送失败，回调三次）<br>Vidu采用回调签名算法进行认证，详情见：\~[回调签名算法](https://platform.vidu.cn/docs/callback-signature)\~ |

```
curl -X POST -H "Authorization: Token {your_api_key}" -H "Content-Type: application/json" -d '
{
    "job_id": "901334076999344129",
    "prompt": "2.5D建模风格，欢快愉悦氛围，近景，男主角在海角天边的沙滩上跳舞，海浪轻拍脚踝，镜头跟随她的旋转动作，展现她笑容和活力。"
}' https://api.vidu.cn/ent/v2/one-click/mv/{your_id}/edit
```

### 响应体

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| id | string | 本次任务vidu生成的任务id |

```
{
  "id": "your_job_id_here"
}
```

## 合成任务接口

### 接口地址：

```
POST https://api.vidu.cn/ent/v2/one-click/mv/{id}/compose
```

### 请求头

| 字段 | 值 | 描述 |
| --- | --- | --- |
| Content-Type | application/json | 数据交换格式 |
| Authorization | Token {your api key} | 将 {your api key} 替换为您的 token |

### 请求体

| 字段 | 必传 | 类型 | 说明 |
| --- | --- | --- | --- |
| id | 是 | String | 任务id，放在path路径中 |
| job_ids | 是 | String | 要合成的分镜列表 |
| callback_url | 可选 | String | Callback 协议<br>需要您在创建任务时主动设置 callback_url，请求方法为 POST，当视频生成任务有状态变化时，Vidu 将向此地址发送包含任务最新状态的回调请求。回调请求内容结构与查询任务API的返回体一致<br>回调返回的"status"包括以下状态：<br>- processing 任务处理中<br>- success 任务完成（如发送失败，回调三次）<br>- failed 任务失败（如发送失败，回调三次）<br>Vidu采用回调签名算法进行认证，详情见：\~[回调签名算法](https://platform.vidu.cn/docs/callback-signature)\~ |

```
curl -X POST -H "Authorization: Token {your_api_key}" -H "Content-Type: application/json" -d '
{
    "job_ids": [
        "id1",
        "id2",
        "id3",
        "id4",
        "id5",
        "id6"
    ]
}' https://api.vidu.cn/ent/v2/one-click/mv/{your_id}/compose
```

### 响应体

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| id | string | 本次任务vidu生成的任务id |

```
{
  "id": "your_job_id_here"
}
```