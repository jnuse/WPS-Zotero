/* 
** A client for the Zotero HTTP integration server.
** (https://www.zotero.org/support/dev/client_coding/http_integration_protocol)
*/

/**
 * Create a client to communicate with Zotero.
 * @param documentId The document ID of the document to be binded with the client.
 * @param processor Word processor interface.
 * @returns client object.
**/
function zc_createClient(documentId, processor) {
    // The proxy server listens on 21931 and forwards requests to 23119
    const commandUrl = 'http://127.0.0.1:21931/connector/document/execCommand';
    const respondUrl = 'http://127.0.0.1:21931/connector/document/respond';

    function requestStatusHint(status) {
        if (status >= 300) {
            if (status < 400) {
                zc_alert(`收到意外的重定向消息 ${status}！`);
            }
            else if (status < 500) {
                zc_alert(`客户端错误 ${status}`);
            }
            else {
                if (status === 503) {
                    zc_alert('Zotero 正在为其他程序提供服务，如果不是这种情况，请重新启动它。');
                }
                else {
                    zc_alert(`服务器错误 ${status}`);
                }
            }
        }
    }

    function execCommand(command) {
        return postRequestXHR(commandUrl, {
            "command": command,
            "docId": documentId,
        })
    }

    function respond(payload) {
        return postRequestXHR(respondUrl, payload);
    }

    /**
     * Send command to Zotero and make changes to documents.
    **/
    function transact(command) {
        // Init transaction
        let state = true;
        processor.init(documentId);

        let flag = false;
        try {
            // Make request
            let req = execCommand(command);
            assert(req);
            flag = true;
            if (req.status < 300) {
                // Keep responding until the transaction is fullfilled
                while (req && req.status < 300) {
                    req = autoRespond(req);
                }
            }
            else {
                state = false;
                if (req) {
                    zc_alert(`来自 Zotero 的意外响应: status = ${req.status}, msg = ${req.payload}`);
                    requestStatusHint(req.status);
                }
            }
        }
        catch (error) {
            state = false;
            zc_alert('发生错误:', error);
            const guide = flag ? '\n您将需要重新启动 Zotero。' : '';
            if (error.name === 'NetworkError') {
                zc_alert('发生网络错误，Zotero 是否正在运行？' + guide);
            }
            else {
                zc_alert(`发生错误 ${error.name}，请单击开发人员工具，导航到控制台并报告问题。` + guide);
            }
        }

        processor.reset(documentId);
        return state;
    }

    // Functions for responding to different word processor commands
    let responders = {
        getActiveDocument: function() {
            return respond({
                "documentID": documentId,
                "outputFormat": "html",
                "supportedNotes": ["footnotes"],
                "supportsImportExport": true,
                "supportsTextInsertion": true,
                "supportsCitationMerging": true,
                "processorName":"Google Docs"
            });
        },

        displayAlert: function(args) {
            const msg = args[1];
            const icon = args[2];
            const buttons = args[3];
            const ret = processor.displayDialog(msg, icon, buttons);
            return respond(ret);
        },

        activate: function(args) {
            const docId = args[0];
            assert(docId === documentId);
            processor.activate(documentId);
            return respond(null);
        },

        canInsertField: function(args) {
            const docId = args[0];
            assert(docId === documentId);
            // TODO: Other cases a field cannot be inserted?
            return respond(!processor.isInLink(documentId));
        },

        setDocumentData: function(args) {
            const docId = args[0];
            const dataStr = args[1];
            assert(docId === documentId);
            let curData = processor.getDocData(documentId);
            if (curData !== dataStr) {
                if (curData) console.debug('update existing document data:', curData);
                processor.setDocData(documentId, dataStr.trim());
            }
            return respond(null);
        },

        getDocumentData: function(args) {
            const defaultDocDataV3 = '<data data-version="3"/>';
            // const defaultDocDataV4 = JSON.stringify("{\"dataVersion\": 4}");
            const docId = args[0];
            assert(docId === documentId);
            let dataStr = processor.getDocData(docId);
            // IMPORTANT: Change bibliographyStyleHasBeenSet to false to always get a setBibliographyStyle command.
            dataStr = dataStr.replaceAll('bibliographyStyleHasBeenSet="1"', 'bibliographyStyleHasBeenSet="0"');
            dataStr = dataStr.replaceAll('\\"bibliographyStyleHasBeenSet\\": true', '\\"bibliographyStyleHasBeenSet\\": false');
            dataStr = dataStr.replaceAll('\\"bibliographyStyleHasBeenSet\\":true', '\\"bibliographyStyleHasBeenSet\\":false');
            // Compatible with MS word integration.
            // default to data version 3
            dataStr = dataStr ? dataStr : defaultDocDataV3;
            return respond(dataStr);
        },

        cursorInField: function(args) {
            const docId = args[0];
            assert(docId === documentId);
            return respond(processor.getFieldsNearCursor(documentId));
        },

        insertField: function(args) {
            const docId = args[0];
            const fieldType = args[1];
            const noteType = args[2];
            assert(docId === documentId);
            assert(fieldType === 'Http');
            if (noteType > 1) {
                zc_alert('仅支持文本内和脚注引文，将改用脚注！');
                noteType = 1;
            }
            const data = processor.insertField(docId, noteType > 0 ? true : false);
            return respond(data);
        },

        insertText: function(args) {
            const docId = args[0];
            const text = args[1];
            assert(docId === documentId);
            processor.insertRich(docId, text);
            return respond(null);
        },

        getFields: function(args) {
            const docId = args[0];
            assert(docId === documentId);
            const data = processor.getFields(docId, true);
            return respond(data);
        },

        convert: function(args) {
            const docId = args[0];
            const fieldIds = args[1];
            const toFieldType = args[2];
            const toNoteType = args[3];
            // Count is undefined
            assert(docId === documentId);
            assert(toFieldType === 'Http');
            processor.convertToNoteType(docId, fieldIds, toNoteType);
            return respond(null);
        },

        // Convert the placeholders in notes to citation fields.
        convertPlaceholdersToFields: function(args) {
            const docId = args[0];
            const placeholderIds = args[1];
            const noteType = args[2];
            assert(docId === documentId);
            assert(noteType < 2);
            const fields = processor.convertPlaceholderLinks(docId, placeholderIds, noteType);
            return respond(fields);
        },

        setBibliographyStyle: function(args) {
            const docId = args[0];
            const firstLineIndent = args[1];
            const indent = args[2];
            const lineSpacing = args[3];
            const entrySpacing = args[4]
            const tabStops = args[5];
            const tabStopsCount = args[6];
            assert(docId === documentId);
            processor.setBibStyle(docId, firstLineIndent, indent, lineSpacing, entrySpacing, tabStops, tabStopsCount);
            return respond(null);
        },

        complete: function(args) {
            delete args;
            // const docId = args[0];
            // assert(docId === documentId);
        },

        delete: function(args) {
            const docId = args[0];
            const fieldId = args[1];
            assert(docId === documentId);
            processor.deleteField(docId, fieldId);
            return respond(null);
        },

        select: function(args) {
            const docId = args[0];
            const fieldId = args[1];
            assert(docId === documentId);
            processor.selectField(docId, fieldId);
            return respond(null);
        },

        removeCode: function(args) {
            const docId = args[0];
            const fieldId = args[1];
            assert(docId === documentId);
            processor.removeFieldCode(docId, fieldId);
            return respond(null);
        },

        setText: function(args) {
            const docId = args[0];
            const fieldId = args[1];
            const text = args[2];
            const isRich = args[3];
            processor.setFieldText(docId, fieldId, text, isRich);
            return respond(null);
        },

        getText: function(args) {
            const docId = args[0];
            const fieldId = args[1];
            const text = processor.getFieldText(docId, fieldId);
            return respond(text);
        },

        setCode: function(args) {
            const docId = args[0];
            assert(docId === documentId);
            const fieldId = args[1];
            const code = args[2];
            processor.setFieldCode(documentId, fieldId, code);
            return respond(null);
        },

        exportDocument: function(args) {
            const docId = args[0];
            // args[1] == Http
            const msg = args[2];
            processor.exportDocument(docId);
            alert(msg);
            return respond(null);
        }
    }

    /**
     * Respond to word processor commands
    **/
    function autoRespond(req) {
        assert(req);
        const payload = req.payload;
        const method = payload.command.split('.')[1];
        const args = Array.from(payload.arguments);
        if (args.length > 0) { 
            var docID = args[0];
            assert(docID === documentId);
        }
        const ret = responders[method](args);
        return ret;
    }

    return {
        id: documentId,
        command: transact,
        export: () => { processor.exportDocument(documentId); },
        import: () => { processor.importDocument(documentId); }
    };
}

    