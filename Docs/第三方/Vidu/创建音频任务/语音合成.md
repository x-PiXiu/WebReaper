# 语音合成

该接口用于输入文本生成对应的朗读音频，可控制朗读的语速、音量、情绪。

该接口是同步接口，不需要回调

## 请求地址

```
POST https://api.vidu.cn/ent/v2/audio-tts
```

## 请求头

| 字段 | 值 | 描述 |
| --- | --- | --- |
| Content-Type | application/json | 数据交换格式 |
| Authorization | Token `{your api key}` | 将`{token}`替换为提供给您的token |

## 请求体

| 参数名称 | 类型 | 必填 | 参数描述 |
| --- | --- | --- | --- |
| text | String | 是 | 需要合成语音的文本<br>1. 长度限制小于 10000 字符<br>2. 段落切换用换行符标记<br>3. 停顿控制：支持自定义文本之间的语音时间间隔，以实现自定义文本语音停顿时间的效果。<br>- 使用方式：在文本中增加&lt;#x#&gt;标记，x 为停顿时长（单位：秒），范围 \[0.01, 99.99\]，最多保留两位小数。文本间隔时间需设置在两个可以语音发音的文本之间，不可连续使用多个停顿标记<br>- 示例：你好&lt;#2#&gt;我是vidu&lt;#2#&gt;很高兴见到你 |
| voice_setting_voice_id | String | 是 | 合成音频的音色id<br>可查看[音色列表](https://shengshu.feishu.cn/sheets/EgFvs6DShhiEBStmjzccr5gonOg)查询全部可用音色 |
| voice_setting_speed | Float | 可选 | 语速，默认为1.0<br>1.0为正常语速，范围 \[0.5,2\]，值为0.5时播报语速最慢，值为2时播报语速最快 |
| voice_setting_volume | Int | 可选 | 音量大小<br>范围 0 - 10，默认为0，代表正常音量，值越大音量越高 |
| voice_setting_pitch | Int | 可选 | 合成音频的语调<br>范围 \[-12,12\]，默认 0，其中 0 为原音色输出 |
| voice_setting_emotion | String | 可选 | 控制合成语音的情绪<br>1. 参数范围 \["happy", "sad", "angry", "fearful", "disgusted", "surprised", "calm"\]，分别对应 7 种情绪：高兴，悲伤，愤怒，害怕，厌恶，惊讶，中性<br>2. 模型会根据输入文本自动匹配合适的情绪，一般无需手动指定 |
| pronunciation_dict_tone | list | 可选 | 定义多音字发音<br>- 定义需要特殊标注的文字或符号对应的注音或发音替换规则，针对多音字场景，在中文文本中，声调用数字表示：一声为 1；二声为 2；三声为 3；四声为 4；轻声为 5。<br>- 示例如下：<br>\["燕少飞/(yan4)(shao3)(fei1)", "达菲/(da2)(fei1)", "omg/oh my god"\] |
| payload | String | 可选 | 透传参数<br>不做任何处理，仅数据传输<br>注：最多 1048576个字符 |

```
curl -X POST -H "Authorization: Token {your_api_key}" -H "Content-Type: application/json" -d '
{
    "text":"你好，欢迎使用vidu开放平台",
    "voice_setting_voice_id": "your_voice_setting_voice_id"
}'https://api.vidu.cn/ent/v2/audio-tts
```

## 响应体

| 参数名称 | 类型 | 描述 |
| --- | --- | --- |
| task_id | String | Vidu生成的任务ID |
| state | String | 处理状态<br>可选值：<br>`queueing` 任务排队中<br>`success` 任务成功<br>`failed` 任务失败 |
| file_url | String | 音频文件url |
| credits | Int | 本次调用使用的积分数 |
| payload | String | 本次调用时传入的透传参数 |
| created_at | String | 任务创建时间 |

```
{
  "task_id": "your_task_id_here",
  "state": "success",
  "file_url": "your_file_url_here",
  "credits": ,
  "payload":"",
  "created_at": "2025-01-01T15:41:31.968916Z"
}
```