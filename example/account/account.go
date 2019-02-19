package account

import (
	"github.com/YiCodes/gosql/sqlcodegen"
)

// 定义模型
type User struct {
	UserID   string
	UserName string
	Sex      byte
}

// 定义实体
var (
	user User
)

// GetUser 获取user.UserID=userID的一条用户
func GetUser(userID string) {
	sqlcodegen.From(user)
	// select User的所有字段
	sqlcodegen.SelectAll(user)
	sqlcodegen.Where(user.UserID == userID)
	sqlcodegen.SetReturnType(sqlcodegen.ReturnRecord)
}

// GetUserList 获取所有user.Sex=0 的用户, 未调用ReturnType，默认返回多条记录（数组）
func GetUserList() {
	sqlcodegen.From(user)
	//指定select User的几个字段
	sqlcodegen.Select(user.UserID, user.UserName)
	sqlcodegen.Where(user.Sex == 0)
}

func GetSortedUserList() {
	sqlcodegen.From(user)
	sqlcodegen.SelectAll(user)
	sqlcodegen.OrderBy(user.Sex)
	sqlcodegen.OrderByDescending(user.UserID)
}

// InsertUser 插入一个用户
func InsertUser() {
	sqlcodegen.InsertAll(user)
}

// UpdateUser 更新user.UserId=userId的用户的UserName和Sex
func UpdateUser(userID string, userName string, sex byte) {
	sqlcodegen.From(user)
	sqlcodegen.Update(user.UserName, userName)
	sqlcodegen.Update(user.Sex, sex)
	sqlcodegen.Where(user.UserID == userID)
}

// DeleteUser 删除一个user.UserId=userId并且user.Sex=0的用户
func DeleteUser(userID string) {
	sqlcodegen.Delete(user)
	sqlcodegen.Where(user.UserID == userID && user.Sex == 0)
}
