# 1.创建主体API

### 请求地址

```Plain
    POST https://api.vidu.cn/ent/v2/subjects
```

### 请求头

| 字段 | 值 | 描述 |
| --- | --- | --- |
| Content-Type | application/json | 数据交换格式 |
| Authorization | Token {your api key} | 将{token}替换为提供给您的token |

### 请求体

| 参数名称 | 类型 | 必填 | 参数描述 |
| --- | --- | --- | --- |
| name | String | 是 | 主体名称（建议唯一） |
| images | Array\[String\] | 可选 | 主体图片，至少上传 1 张主体图片 <br><br>注1：支持传入图片 Base64 编码或图片URL（确保可访问）； <br>注2：最多支持输入 3 张图；<br>注3：图片支持 png、jpeg、jpg、webp格式； <br>注4：图片比例需要小于 1:4 或者 4:1 ； <br>注5：请注意，base64 decode之后的字节长度需要小于20M，且编码必须包含适当的内容类型字符串，例如：<br><br>`data:image/png;base64,{base64_encode}` |
| videos | Array\[String\] | 可选 | 视频参考支持上传 1 个主体视频 <br><br>注1：仅参考生viduq2-pro模型支持使用视频主体 <br>注2：最多支持上传 **1个5秒** 的视频 <br>注3：视频支持 mp4、avi、mov格式<br>注4：视频像素不能小于 128\*128，且比例需要小于1:4或者4:1 注5：请注意，base64 decode之后的字节长度需要小于20M，且编码必须包含适当的内容类型字符串，例如：<br><br>`data:video/mp4;base64,{base64_encode}` |
| voice_id | String | 否 | 主体音色Id，该信息仅在创建音视频直出任务时使用<br><br>注1：不传音色id 生成音视频直出任务时，系统会自动推荐音色<br>注2：**q2-pro**及**q3**相关模型不支持使用音色id |

### 响应体

| 字段 | 类型 | 描述 |
| --- | --- | --- |
| id | String | 本次生成的主体id（server_id） |
| name | String | 本次生成自定义的主体名称 |
| images | Array\[String\] | 本次生成主体的图像参数 |
| videos | Array\[String\] | 本次生成主体的视频参数 |
| voice_id | String | 本次生成主体绑定的音色id |
| style | String | 本次生成的主体风格信息 |
| description | String | 本次生成的主体描述 |
| credits | Int | 本次调用使用的积分数 |

# 2.查询主体API

### 请求地址

```Plain
    GET https://api.vidu.cn/ent/v2/subjects
```

### 请求头

| 字段 | 值 | 描述 |
| --- | --- | --- |
| Content-Type | application/json | 数据交换格式 |
| Authorization | Token {your api key} | 将{token}替换为提供给您的token |

### 请求体

| 参数名称 | 类型 | 必填 | 参数描述 |
| --- | --- | --- | --- |
| ownership | String | 非必填 | 主体归属，枚举值为：<br><br>- system：官方主体<br>- private：个人主体（默认值） |
| subject_ids | array | 非必填 | 指定主体id查询，可以查询多个 |
| count | Int | 非必填 | 查询数量，默认20条，最多100条 |
| next_page_token | String | 非必填 | 第一次next_page_token可以不传为空，如果有多页，则会返回next_page_token，下一页用 next_page_token 来请求 |

### 响应体

| 字段 | 子字段 | 类型 | 描述 |
| --- | --- | --- | --- |
| subjects |  | list | 本次查询的主体信息 |
|  | id | String | 主体id（server_id） |
|  | name | String | 主体名称 |
|  | images | Array\[String\] | 主体的图像参数 |
|  | videos | Array\[String\] | 主体的视频参数 |
|  | voice_id | String | 主体绑定的音色id |
|  | style | String | 主体的风格信息（官方主体不展示该信息） |
|  | description | String | 主体的详细描述信息（官方主体不展示该信息） |
|  | videos | String | 主体的视频参数 |
| next_page_token |  | String | 是否有下一页 |
| count |  | Int | 默认20，最大100条 |

# 3. 编辑主体API

### 请求地址

```Plain
    PUT https://api.vidu.cn/ent/v2/subjects/{id}
```

### 请求头

| 字段 | 值 | 描述 |
| --- | --- | --- |
| Content-Type | application/json | 数据交换格式 |
| Authorization | Token {your api key} | 将{token}替换为提供给您的token |

### 请求体

| 参数名称 | 类型 | 必填 | 参数描述 |
| --- | --- | --- | --- |
| id | String | 是 | vidu生成的主体id（server_id） |
| name | String | 可选 | 新的主体名称，64字符 |
| images | Array\[String\] | 可选 | 主体图片，**直接覆盖原主体图片** <br>注1：支持传入图片 Base64 编码或图片URL（确保可访问）； <br>注2：最多支持输入 3 张图； <br>注3：图片支持 png、jpeg、jpg、webp格式； <br>注4：图片比例需要小于 1:4 或者 4:1 ； <br>注5：请注意，base64 decode之后的字节长度需要小于20M，且编码必须包含适当的内容类型字符串，例如：<br><br>`data:image/png;base64,{base64_encode}` |
| videos | Array\[String\] | 可选 | 视频参考支持上传 1 个主体视频，**直接覆盖原主体视频，同时存在图片和视频的情况下，则仅视频生效**<br><br>注1：该参数仅对创建时有参考视频的主体生效 <br>注2：仅参考生viduq2-pro模型支持使用视频主体 <br>注3：最多支持上传 **1个5秒** 的视频 <br>注4：视频支持 mp4、avi、mov格式 <br>注5：视频像素不能小于 128\*128，且比例需要小于1:4或者4:1 <br>注6：请注意，base64 decode之后的字节长度需要小于20M，且编码必须包含适当的内容类型字符串，例如：<br><br>`data:video/mp4;base64,{base64_encode}` |
| style | String | 可选 | 新的主体风格描述，100字符 |
| description | String | 可选 | 新的主体的详细描述信息，2000字符 |
| voice_id | String | 可选 | 新的主体音色Id |

### 响应体

| 字段 | 类型 | 描述 |
| --- | --- | --- |
| id | String | 本次修改的主体id |
| name | String | 新的主体名称 |
| style | String | 新的主体风格描述 |
| description | String | 新的主体的详细描述信息 |
| voice_id | String | 新的主体音色Id |

# 4.参考生视频使用说明

### 请求地址

```JSON
        POST https://api.vidu.cn/ent/v2/reference2video
```

### 请求头

| 字段 | 值 | 描述 |
| --- | --- | --- |
| Content-Type | application/json | 数据交换格式 |
| Authorization | Token {your api key} | 将{token}替换为提供给您的token |

### 请求体

| 参数名称 | 子参数 | 类型 | 必填 | 参数描述 |
| --- | --- | --- | --- | --- |
| model |  | String | 是 | 模型名称可选值：viduq2-pro、viduq2、viduq1、vidu2.0<br><br>viduq2-pro：支持参考视频，支持编辑视频<br>viduq2：动态效果好，生成细节丰富 <br>viduq1：画面清晰，平滑转场，运镜稳定 - <br>vidu2.0：生成速度快 |
| subjects |  | List\[Array\] | 是 | \- 使用q2、q1、2.0模型时，只能使用图片主体和文字主体<br>- 图片或文字主体最多不超过7个<br>- 使用q2-pro模型时，可以使用视频主体、文字主体和图片主体<br>- 图片或文字主体最多不超过4个 <br>- 视频主体最多不超过2个 |
|  | name | String | 是 | 用户指定的主体名称（name），生成时可以通过`[@name]`的方式使用，例如：`[@图1]` |
|  | server_id | String | 可选 | 通过创建主体API获取的 主体id |
| prompt |  | String | 是 | 文本提示词生成视频的文本描述。 <br>注1：字符长度不能超过 5000 个字符 <br>注2：通过`[@subjects_name]` 来表示主体内容，<br>例如："@1 和 @2 在一起吃火锅，并且旁白音说火锅大家都爱吃。" |
| audio |  | Bool | 可选 | 是否使用音视频直出能力，默认false，可选值 true、false - true：使用音视频直出能力。 - false：不使用音视频直出能力。 |
| duration |  | Int | 可选 | 视频时长参数，默认值依据模型而定：<br><br>viduq2-pro：默认5秒，可选：0-10（0秒时为自动推荐） viduq2：默认5秒，可选：1-10 <br>viduq1：默认5秒，可选：5 vidu2.0：默认4秒，可选：4 |
| seed |  | Int | 可选 | 随机种子当默认不传或者传0时，会使用随机数替代手动设置则使用设置的种子 |
| aspect_ratio |  | String | 可选 | 比例默认 16:9，可选值如下：16:9、9:16、4:3、3:4、1:1 注：4:3、3:4仅支持q2模型 |
| resolution |  | String | 可选 | 分辨率参数，默认值依据模型和视频时长而定： viduq2、viduq2-pro（1-10秒）：默认 720p, 可选：540p、720p、1080p <br>viduq1 （5秒）：默认 1080p, 可选：1080p <br>vidu2.0 （4秒）：默认 360p, 可选：360p、720p |
| movement_amplitude |  | String | 可选 | 运动幅度默认 auto，可选值：auto、small、medium、large 注：使用q2模型时该参数不生效 |
| payload |  | String | 可选 | 透传参数不做任何处理，仅数据传输注：最多 1048576个字符 |
| off_peak |  | Bool | 可选 | 错峰模式，默认为：false，可选值： - true：错峰生成视频； - false：即时生成视频； <br><br>注1：错峰模式消耗的积分更低，具体请查看产品定价 <br>注2：错峰模式下提交的任务，会在48小时内生成，未能完成的任务会被自动取消，并返还该任务的积分； <br>注3：您也可以[手动取消](https://platform.vidu.cn/docs/cancel-task-api)错峰任务 |
| watermark |  | Bool | 可选 | 是否添加水印 - true：添加水印； - false：不添加水印； <br><br>注1：目前水印内容为固定，内容由AI生成，默认不加 <br>注2：您可以通过watermarked_url参数查询获取带水印的视频内容，详情见查询任务接口 |
| wm_position |  | Int | 可选 | 水印位置，表示水印出现在图片的位置，默认为：3，<br>可选项为： <br>1：左上角 <br>2：右上角 <br>3：右下角 <br>4：左下角 |
| wm_url |  | String | 可选 | 水印内容，此处为图片URL不传时，使用默认水印：内容由AI生成 |
| meta_data |  | String | 可选 | 元数据标识，json格式字符串，透传字段，您可以 自定义格式 或使用 示例格式 ，示例如下：<br>{ "Label": "your_label","ContentProducer": "yourcontentproducer","ContentPropagator": "your_content_propagator","ProduceID": "yourproductid", "PropagateID": "your_propagate_id","ReservedCode1": "yourreservedcode1", "ReservedCode2": "your_reserved_code2" } 该参数为空时，默认使用vidu生成的元数据标识 |
| callback_url |  | String | 可选 | Callback 协议 需要您在创建任务时主动设置 callback_url，请求方法为 POST，当视频生成任务有状态变化时，Vidu 将向此地址发送包含任务最新状态的回调请求。回调请求内容结构与查询任务API的返回体一致 回调返回的"status"包括以下状态： - processing 任务处理中 - success 任务完成（如发送失败，回调三次） - failed 任务失败（如发送失败，回调三次） Vidu采用回调签名算法进行认证，详情见：[回调签名算法](https://platform.vidu.cn/docs/callback-signature) |

使用主体：

```JSON
        curl -X POST -H "Authorization: Token {your_api_key}" -H "Content-Type: application/json" -d '
        {
            "model": "viduq2",
            "subjects": [
                {
                    "name": "1",
                    "server_id": "123123"
                },
                {
                    "name": "2",
                    "server_id": "321321"
                },
                {
                    "name": "3",
                    "server_id": "456456"
                }
            ],
            "prompt": "[@1]和[@2]在[@3]一起吃火锅",
            "duration": 8,
            "audio":false
        }' https://api.vidu.cn/ent/v2/reference2video
```

### 响应体

| 字段 | 类型 | 描述 |
| --- | --- | --- |
| task_id | String | Vidu 生成的任务ID |
| state | String | 处理状态 可选值： created 创建成功 queueing 任务排队中 processing 任务处理中 success 任务成功 failed 任务失败 |
| model | String | 本次调用的模型名称 |
| prompt | String | 本次调用的提示词参数 |
| images | Array\[String\] | 本次调用的图像参数 |
| duration | Int | 本次调用的视频时长参数 |
| seed | Int | 本次调用的随机种子参数 |
| aspect_ratio | String | 本次调用的 比例 参数 |
| resolution | String | 本次调用的分辨率参数 |
| bgm | Bool | 本次调用的背景音乐参数 |
| audio | Bool | 本次调用是否开启音视频直出 |
| movement_amplitude | String | 本次调用的镜头动态幅度参数 |
| payload | String | 本次调用时传入的透传参数 |
| off_peak | Bool | 本次调用时是否使用错峰模式 |
| credits | Int | 本次调用使用的积分数 |
| watermark | Bool | 本次提交任务是否使用水印 |
| created_at | String | 任务创建时间 |

```JSON
        {
          "task_id": "your_task_id_here",
          "state": "created",
          "model": "viduq2",
          "prompt": "[@1]和[@2]在[@3]一起吃火锅",
          "duration": 8,
          "seed": random_number,
          "aspect_ratio": "3:4",
          "resolution": "1080p",
          "movement_amplitude": "auto",
          "payload":"",
          "off_peak": false,
          "audio":false,
          "credits":credits_number,
          "created_at": "2025-01-01T15:41:31.968916Z"
        }
```