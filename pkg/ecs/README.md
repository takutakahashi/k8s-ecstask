# XPod to ECS Task Definition Converter

このパッケージは、Kubernetes Pod仕様（XPod spec）をAmazon ECS Task Definitionに変換するGoライブラリです。シークレットとConfigMapはAWS Systems Manager Parameter Storeに保管されることを前提としています。

## 特徴

- ✅ 基本的なPod仕様をECS Task Definitionに変換
- ✅ コンテナリソース（CPU、メモリ）の変換
- ✅ ポートマッピングの変換
- ✅ 環境変数の変換
- ✅ シークレットとConfigMapをParameter Storeパスに変換
- ✅ ボリュームマウントの変換（ホストパス、EmptyDir）
- ✅ タグの変換
- ✅ ロググループの設定
- ❌ InitContainers（ECSでサポートされていない）
- ❌ 複雑なボリュームタイプ（Secret、ConfigMapボリューム等）

## インストール

```bash
go get github.com/takutakahashi/k8s-ecstask/pkg/ecs
```

## 基本的な使用方法

```go
package main

import (
    "encoding/json"
    "fmt"
    "log"

    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/api/resource"

    "github.com/takutakahashi/k8s-ecstask/pkg/ecs"
)

func main() {
    // コンバーターの作成
    options := ecs.ConversionOptions{
        ParameterStorePrefix:    "/myapp",
        DefaultExecutionRoleArn: "arn:aws:iam::123456789012:role/ecsTaskExecutionRole",
        DefaultTaskRoleArn:      "arn:aws:iam::123456789012:role/ecsTaskRole",
        DefaultLogDriver:        "awslogs",
        DefaultLogOptions: map[string]string{
            "awslogs-group":  "/ecs/myapp",
            "awslogs-region": "ap-northeast-1",
        },
        SkipUnsupportedFeatures: true,
    }

    converter := ecs.NewConverter(options)

    // XPod仕様の定義
    xpodSpec := &ecs.XPodSpec{
        Family: "web-app",
        CPU:    "256",
        Memory: "512",
        Containers: []corev1.Container{
            {
                Name:  "nginx",
                Image: "nginx:alpine",
                Ports: []corev1.ContainerPort{
                    {ContainerPort: 80, Protocol: corev1.ProtocolTCP},
                },
                Resources: corev1.ResourceRequirements{
                    Limits: corev1.ResourceList{
                        corev1.ResourceCPU:    resource.MustParse("256m"),
                        corev1.ResourceMemory: resource.MustParse("512Mi"),
                    },
                },
                Env: []corev1.EnvVar{
                    {Name: "ENV", Value: "production"},
                    {
                        Name: "DB_PASSWORD",
                        ValueFrom: &corev1.EnvVarSource{
                            SecretKeyRef: &corev1.SecretKeySelector{
                                LocalObjectReference: corev1.LocalObjectReference{
                                    Name: "db-credentials",
                                },
                                Key: "password",
                            },
                        },
                    },
                },
            },
        },
    }

    // ECS Task Definitionに変換
    taskDef, err := converter.Convert(xpodSpec)
    if err != nil {
        log.Fatalf("変換に失敗しました: %v", err)
    }

    // JSONとして出力
    jsonBytes, _ := json.MarshalIndent(taskDef, "", "  ")
    fmt.Println(string(jsonBytes))
}
```

## Parameter Store マッピング

シークレットとConfigMapは以下のようにParameter Storeパスにマッピングされます：

- **Secret**: `{ParameterStorePrefix}/secrets/{secretName}/{key}`
- **ConfigMap**: `{ParameterStorePrefix}/configmaps/{configMapName}/{key}`

例：
- Secret `db-credentials` の `password` キー → `/myapp/secrets/db-credentials/password`
- ConfigMap `app-config` の `config.yaml` キー → `/myapp/configmaps/app-config/config.yaml`

## コンバージョンオプション

| オプション | 説明 | デフォルト値 |
|-----------|------|------------|
| `ParameterStorePrefix` | Parameter Storeのプレフィックス | `/xpod` |
| `DefaultLogDriver` | デフォルトのログドライバー | `awslogs` |
| `DefaultLogOptions` | デフォルトのログオプション | `awslogs-group: /ecs/task, awslogs-region: us-east-1` |
| `SkipUnsupportedFeatures` | サポートされていない機能をスキップ | `false` |
| `DefaultExecutionRoleArn` | デフォルトの実行ロールARN | 空文字 |
| `DefaultTaskRoleArn` | デフォルトのタスクロールARN | 空文字 |

## サポートされる機能

### ✅ サポート済み

- **Container specs**: 基本的なコンテナ仕様
- **Resource requirements**: CPU、メモリの制限と要求
- **Port mappings**: コンテナポートのマッピング
- **Environment variables**: 環境変数の設定
- **Secrets/ConfigMaps**: Parameter Storeへの変換
- **Volume mounts**: HostPathとEmptyDirボリューム
- **Logging**: CloudWatch Logsの設定
- **Tags**: リソースタグの設定

### ❌ サポートされていない機能

- **InitContainers**: ECSでは直接サポートされていない
- **Secret/ConfigMap volumes**: Parameter Storeを使用してください
- **Field references**: `fieldRef`, `resourceFieldRef`
- **Complex volume types**: PVC、CSI等

## テスト

```bash
go test ./pkg/xpodtoecs/...
```

## 貢献

バグレポートや機能改善の提案は歓迎します。プルリクエストを送信する前にテストが通ることを確認してください。

## ライセンス

Apache License 2.0