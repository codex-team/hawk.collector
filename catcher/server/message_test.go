package server

import (
	"testing"
	"sync"
	"github.com/codex-team/hawk.catcher/catcher/lib"
	"github.com/stretchr/testify/assert"
)

func TestValidProcessMessage(t *testing.T) {
	var wg sync.WaitGroup
	msg := make(chan lib.Message, 1)
	body := []byte(`{"token":"eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9","sender":{"ip":"127.0.0.1"},"catcher_type":"errors/golang","payload":{"title":"Field is missing","timestamp":1545203808,"severity":16,"backtrace":[{"file":"/var/www/codex/vendor/codex-team/editor.js/EditorJS/EditorJS.php","line":77,"source code":[{"line number":76,"content":"         if (!isset($data['blocks'])) {"},{"line number":77,"content":"             throw new EditorJSException('Field  is missing');"},{"line number":78,"content":"         }"}]},{"file":"/var/www/codex/application/classes/Controller/Articles/Index.php","called line":"191","source code":[{"line number":"190","content":"     {"},{"line number":"191","content":"         $editor = new EditorJS($content, Model_Article::getEditorConfig());"},{"line number":"192","content":"         $blocks = $editor->getBlocks();"}]}],"get":{},"post":{"text":"Hello, World!","is_published":false},"headers":{},"source release":""}}`)
	expectedPayload := []byte(`{"token":"eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9","payload":{"title":"Field is missing","timestamp":1545203808,"severity":16,"backtrace":[{"file":"/var/www/codex/vendor/codex-team/editor.js/EditorJS/EditorJS.php","line":77,"source code":[{"line number":76,"content":"         if (!isset($data['blocks'])) {"},{"line number":77,"content":"             throw new EditorJSException('Field  is missing');"},{"line number":78,"content":"         }"}]},{"file":"/var/www/codex/application/classes/Controller/Articles/Index.php","called line":"191","source code":[{"line number":"190","content":"     {"},{"line number":"191","content":"         $editor = new EditorJS($content, Model_Article::getEditorConfig());"},{"line number":"192","content":"         $blocks = $editor-\u003egetBlocks();"}]}],"get":{},"post":{"text":"Hello, World!","is_published":false},"headers":{},"source release":""}}`)

	wg.Add(1)
	go func(ch <-chan lib.Message) {
		defer wg.Done()
		message := <-messagesQueue
		msg <- message
	}(messagesQueue)
	response := processMessage(body)

	wg.Wait()
	assert.Equal(t, <-msg, lib.Message{expectedPayload, "errors/golang"})
	assert.Equal(t, response, Response{false, "OK", 200})
}

func TestInvalidProcessMessage(t *testing.T) {
	assert.Equal(t, processMessage([]byte("")), Response{true, "Invalid JSON format", 400})
	assert.Equal(t, processMessage([]byte("{}")), Response{true, "Token is empty", 400})
}