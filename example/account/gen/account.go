package account

import (
	"context"
	"github.com/YiCodes/gosql/sqlutil"
	"database/sql"
)

// 定义模型
type User struct {
	UserID   string
	UserName string
	Sex      byte
}

// GetUser 获取user.UserID=userID的一条用户
func GetUser(db sqlutil.DbObject, userID string) (*User, error) {
	const query = "SELECT UserID, UserName, Sex\nFROM User\nWHERE UserID = ?\n"
	rows, err := db.QueryContext(context.Background(), query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if rows.Next() {
		var o = new(User)
		rows.Scan(&o.UserID, &o.UserName, &o.Sex)
		return o, nil
	}
	return nil, nil
}
// GetUserList 获取所有user.Sex=0 的用户, 未调用ReturnType，默认返回多条记录（数组）
func GetUserList(db sqlutil.DbObject) ([]*User, error) {
	const query = "SELECT UserID, UserName\nFROM User\nWHERE Sex = 0\n"
	rows, err := db.QueryContext(context.Background(), query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*User
	for rows.Next() {
		var o = new(User)
		rows.Scan(&o.UserID, &o.UserName)
		result = append(result, o)
	}
	return result, nil
}
func GetSortedUserList(db sqlutil.DbObject) ([]*User, error) {
	const query = "SELECT UserID, UserName, Sex\nFROM User\nORDER BY Sex,UserID DESC\n"
	rows, err := db.QueryContext(context.Background(), query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*User
	for rows.Next() {
		var o = new(User)
		rows.Scan(&o.UserID, &o.UserName, &o.Sex)
		result = append(result, o)
	}
	return result, nil
}
func InsertUser(db sqlutil.DbObject, o *User) (sql.Result, error) {
	const query = "INSERT INTO User(UserID,UserName,Sex)\nVALUES(?,?,?)"
	return db.ExecContext(context.Background(), query,o.UserID,o.UserName,o.Sex)
}
// UpdateUser 更新user.UserId=userId的用户的UserName和Sex
func UpdateUser(db sqlutil.DbObject, userID string, userName string, sex byte) (sql.Result, error) {
	const query = "UPDATE User\nSET UserName = ?,Sex = ?\nWHERE UserID = ?\n"
	return db.ExecContext(context.Background(), query, userName, sex, userID)
}
// DeleteUser 删除一个user.UserId=userId并且user.Sex=0的用户
func DeleteUser(db sqlutil.DbObject, userID string) (sql.Result, error) {
	const query = "DELETE User\nWHERE UserID = ? AND Sex = 0\n"
	return db.ExecContext(context.Background(), query, userID)
}
