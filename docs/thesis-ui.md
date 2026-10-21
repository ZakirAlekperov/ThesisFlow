# Thesis browser form / Форма темы ВКР

The student materials page renders the thesis topic and order reference using
Go `html/template`. POST `/groups/{group}/students/{student}/topic` checks
that the student belongs to the group, validates CSRF and requires a nonempty
topic and order reference. Changing an existing topic with the same order
returns HTTP 409; a new order is required. Successful changes redirect to
the student page; errors do not expose internal SQL details.

Страница материалов ВКР выводит тему и реквизиты приказа через
`html/template`. При сохранении проверяются принадлежность студента группе,
CSRF-токен, непустые тема и приказ. Для изменения темы необходимо указать
новые реквизиты приказа (HTTP 409 при повторном использовании старых).
После успешной записи происходит переход обратно на страницу студента.
