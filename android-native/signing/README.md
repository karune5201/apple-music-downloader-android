# 签名密钥（更新必须沿用同一把）

App 升级能不能「覆盖安装」只取决于签名证书是否一致：证书一变，Android 会拒绝
`INSTALL_FAILED_UPDATE_INCOMPATIBLE`，只能先卸载（连带清掉 App 数据）。

历史上签名密钥放在 `build/` 里，而 `build-apk.sh` 每次开头都 `rm -rf build`，
于是**每次构建都会重新生成一把新钥匙** —— 这就是“每次更新都得卸载”的根因。

> **注意**：本仓库若公开，这把密钥也会公开。公开的签名密钥意味着别人可以签出
> 与你的 App 同签名、从而能覆盖安装的包。自用/内部传播无碍；如果你在意，
> 请改用私有仓库，或换成自己的发布密钥（见文末）。

现在密钥固定放在 `signing/amdl-release.keystore`，并被仓库跟踪（不再落在 `build/` 里，
也不再被 `.gitignore` 忽略）。**删掉它 = 以后所有更新都必须卸载重装**，请不要再移动或重命名。

| 项 | 值 |
|---|---|
| 文件 | `signing/amdl-release.keystore`（PKCS12） |
| 别名 | `androiddebugkey` |
| storepass / keypass | `android`（可用环境变量覆盖，见下） |
| 证书 SHA-256 | `C0:BA:62:FF:FE:60:03:78:79:A5:0B:16:7F:A3:70:92:8F:D3:89:EC:3F:36:F4:EA:9A:30:0D:25:1A:04:12:D8` |

指纹同时写在 `signing/fingerprint.txt`，`build-apk.sh` 构建完会比对，不一致会告警。

## 覆盖安装

- 用本仓库脚本构建 → 同一把钥匙 → `adb install -r out/app-release.apk` 直接升级，不需要卸载；
- 想换成自己的发布密钥：`AMDL_KEYSTORE=/path/release.jks AMDL_KEYSTORE_PASSWORD=... AMDL_KEY_ALIAS=... ./build-apk.sh`，
  换钥匙后**必须卸载一次**旧版本，之后又能连续覆盖升级（新指纹请写进 `fingerprint.txt`）。

## 自建密钥（可选）

```bash
keytool -genkeypair -v -keystore signing/amdl-release.keystore \
  -storepass android -keypass android -alias androiddebugkey \
  -keyalg RSA -keysize 2048 -validity 10000 -dname "CN=AMDL,O=AMDL,C=US"
```

`--validity 10000`（约 27 年）是为了避免证书过期导致以后装不上；Android 只校验
证书一致性，不校验自签名证书的签发者。
