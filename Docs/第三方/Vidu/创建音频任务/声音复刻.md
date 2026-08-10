# 声音复刻

该接口可用于复刻真人音色，并使用该音色朗读指定文本。

该接口是同步接口，不需要回调。

## 请求地址

```
POST https://api.vidu.cn/ent/v2/audio-clone
```

## 请求头

| 字段 | 值 | 描述 |
| --- | --- | --- |
| Content-Type | application/json | 数据交换格式 |
| Authorization | Token `{your api key}` | 将`{token}`替换为提供给您的token |

## 请求体

| 参数名称 | 类型 | 必填 | 参数描述 |
| --- | --- | --- | --- |
| audio_url | String | 是 | 原音频 URL（需要确保可访问）<br>模型将以此参数中传入的音频音色为示例对音色进行复刻<br>注1：视频仅支持格式：mp3、m4a、wav；<br>注2：上传的音频文件的时长最少应不低于 10 秒，最长应不超过 5 分钟；<br>注3：上传的音频文件大小需不超过20mb；<br>注4：音频内容免涉版权，否则会被下架或销毁 |
| voice_id | String | 是 | 自定义的声音ID，示例："vidu01"，用户进行自定义 voice_id 时需注意：<br>- 自定义的 voice_id 长度范围\[8,256\]<br>- 首字符必须为英文字母<br>- 允许数字、字母、横线、下划线<br>- 末位字符不可为 -、\_\*<br>- voice_id 不可与已有 id 重复，否则会报错 |
| prompt_audio_url | String | 可选 | 音色复刻示例音频<br>提供本参数将有助于增强语音合成的音色相似度和稳定性，若使用本参数，需同时上传一小段示例音频。<br>注1：视频仅支持格式：mp3、m4a、wav；<br>注2：上传的音频文件的时长最少应小于 8 秒；<br>注3：上传的音频文件大小需不超过20mb |
| prompt_text | String | 可选 | 示例音频对应的文本内容<br>需确保和音频内容一致，句末需有标点符号做结尾 |
| text | String | 是 | 复刻试听参数<br>限制 1000 字符以内，模型将使用复刻后的音色朗读本段文本内容，并返回试听音频链接。<br>注：试听将根据字符数正常收取语音合成费用 |
| payload | String | 可选 | 透传参数<br>不做任何处理，仅数据传输<br>注：最多 1048576个字符 |

```
curl -X POST -H "Authorization: Token {your_api_key}" -H "Content-Type: application/json" -d '
{
    "audio_url": "your_audio_url",
    "voice_id": "your_voice_id",
    "text":"你好，欢迎使用vidu开放平台"
}'https://api.vidu.cn/ent/v2/audio-clone
```

## 响应体

| 参数名称 | 类型 | 描述 |
| --- | --- | --- |
| task_id | String | Vidu生成的任务ID |
| state | String | 处理状态<br>可选值：<br>- `queueing` 任务排队中<br>- `success` 任务成功<br>- `failed` 任务失败 |
| voice_id | String | 用户自定义的 voice_id，如果任务失败则不返回 |
| demo_audio | String | 如果请求体中传入了试听文本 text，那么本参数将以链接形式返回试听音频，否则本参数为空值 |
| payload | String | 本次调用时传入的透传参数 |
| created_at | String | 任务创建时间 |

```
{
  "task_id": "your_task_id_here",
  "state": "success",
  "voice_id": "your_voice_id_here",
  "demo_audio": "your_demo_audio_here",
  "payload":"",
  "created_at": "2025-01-01T15:41:31.968916Z"
}
```

## **注意：**

产出的快速复刻音色为临时音色，若希望永久保留某复刻音色，请于 168 小时（7 天）内在[语音合成](https://platform.vidu.cn/docs/text-to-speech)接口中调用该音色（不包含本接口内的试听行为），若超过时限，该音色将被删除，复刻所消耗的积分不会退还。