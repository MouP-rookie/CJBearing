// 文件说明：仪表接入层时间函数，集中时间来源便于测试替换。

package instrument

import "time"

// timeNow 集中提供时间来源，便于测试替换仪表接收时间。
func timeNow() time.Time { return time.Now() }
