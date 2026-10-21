package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestThesisTopicFormRequiresOrder checks the browser workflow: assigning a
// topic to one student's existing thesis requires approval order metadata.
// RU: Проверяет создание темы ВКР через форму только с реквизитами приказа.
func TestThesisTopicFormRequiresOrder(t *testing.T) {
	h,groupID,studentID,_,_,_:=documentFixture(t)
	page:=documentURL(groupID,studentID)
	token:=documentToken(t,h,page)
	submit:=func(topic,order,csrf string)*httptest.ResponseRecorder{
		t.Helper()
		values:=url.Values{"topic":{topic},"order_reference":{order},"csrf_token":{csrf}}
		req:=httptest.NewRequest(http.MethodPost,fmt.Sprintf("%s/topic",page),strings.NewReader(values.Encode()))
		req.Header.Set("Content-Type","application/x-www-form-urlencoded")
		rec:=httptest.NewRecorder();h.ServeHTTP(rec,req)
		return rec
	}
	if got:=submit("Разработка приложения","","bad");got.Code!=http.StatusForbidden{
		t.Errorf("forged token status=%d",got.Code)
	}
	if got:=submit("Разработка приложения","",token);got.Code!=http.StatusBadRequest{
		t.Errorf("missing order status=%d: %s",got.Code,got.Body.String())
	}
	if got:=submit("Разработка приложения","Приказ № 25",token);got.Code!=http.StatusSeeOther{
		t.Fatalf("assignment status=%d: %s",got.Code,got.Body.String())
	}
	rec:=httptest.NewRecorder();h.ServeHTTP(rec,httptest.NewRequest(http.MethodGet,page,nil))
	for _,want:=range []string{"Разработка приложения","Приказ № 25","Тема ВКР"}{
		if !strings.Contains(rec.Body.String(),want){t.Errorf("missing %q on student page",want)}
	}
	if got:=submit("Другая тема","Приказ № 25",token);got.Code!=http.StatusConflict{
		t.Errorf("reused order status=%d",got.Code)
	}
	if got:=submit("Другая тема","Приказ № 26",token);got.Code!=http.StatusSeeOther{
		t.Errorf("new order status=%d: %s",got.Code,got.Body.String())
	}
}
