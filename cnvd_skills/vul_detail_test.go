package cnvd_skills

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCnvdSkills_RequestVulDetail(t *testing.T) {
	proxyProvider := FixedProxyProvider("http://121.206.45.124:64257")
	//proxyProvider := PinYiProxyProvider
	detail, err := NewCnvdSkills().RequestVulDetailByID("CNVD-2021-67823", proxyProvider)
	assert.Nil(t, err)
	marshal, err := json.Marshal(detail)
	assert.Nil(t, err)
	fmt.Println("抓取结果： " + string(marshal))
	// Example:
	// {
	//    "CNVD": "CNVD-2021-67823",
	//    "CVE": "CVE-2021-39148",
	//    "PublishTimeStr": "2021-09-03",
	//    "HazardLevel": {
	//        "Level": "中",
	//        "CVSS2": "AV:N/AC:M/Au:S/C:P/I:P/A:P"
	//    },
	//    "Product": "XStream XStream <=1.4.17",
	//    "Description": "XStream是一个开源Java类库，主要用于将对象序列化成XML（JSON）或反序列化为对象。\n\t\t\t\t\t\t\t\t\t\t\t\n\t\t\t\t\t\t\t\t\t\t\t\t\n\t\t\t\t\t\t\t\t\t\t\t\n\t\t\t\t\t\t\t\t\t\t\t\tXStream 1.4.17及之前版本存在任意代码执行漏洞，攻击者可利用该漏洞导致任意代码执行。",
	//    "Category": "通用型漏洞",
	//    "Reference": "http://x-stream.github.io/changes.html",
	//    "FixPlan": "厂商已发布了漏洞修复程序，请及时关注更新：\n\n\t\t\t\t\t\t\t\t\t\t\t\n\t\t\t\t\t\t\t\t\t\t\t\thttp://x-stream.github.io/changes.html",
	//    "VendorPatchHTML": "\n\t\t\t\t\t\t\t\t\t\t\t\n\t\t\t\t\t\t\t\t\t\t\t\n\t\t\t\t\t\t\t\t\t\t\t\t<a href=\"/patchInfo/show/289241\">XStream任意代码执行漏洞（CNVD-2021-67823）的补丁</a>\n\t\t\t\t\t\t\t\t\t\t\t\n\t\t\t\t\t\t\t\t\t\t",
	//    "Validate": "(暂无验证信息)",
	//    "PostTimeStr": "2021-08-23",
	//    "RecordTimeStr": "2021-09-03",
	//    "UpdateTimeStr": "2021-09-03",
	//    "AttachFile": "(无附件)"
	// }

}
