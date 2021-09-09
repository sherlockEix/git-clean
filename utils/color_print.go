package utils

import "github.com/fatih/color"

var BlueFunc, BlueLnFunc, RedFunc, RedlnFunc func(a ...interface{})
var blueColor, redColor *color.Color

func init() {
	blueColor = color.New(color.FgBlue).Add(color.Bold)
	BlueFunc = blueColor.PrintFunc()
	BlueLnFunc = blueColor.PrintlnFunc()
	redColor = color.New(color.FgHiRed).Add(color.Bold)
	RedFunc = redColor.PrintFunc()
	RedlnFunc = redColor.PrintlnFunc()
}

func BlueFStr(format string, a interface{}) string {
	return blueColor.Sprintf(format, a)
}
func BlueStr(a interface{}) string {
	return blueColor.Sprint(a)
}
func RedFStr(format string, a interface{}) string {
	return redColor.Sprintf(format, a)
}
func RedStr(a interface{}) string {
	return redColor.Sprint(a)
}
