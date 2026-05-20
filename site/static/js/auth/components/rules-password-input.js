class Rule {
    /** @type {HTMLElement} */
    self
    /** @type {HTMLElement} */
    icon
    /** @type {RegExp} */
    regex
    /** @type {Boolean} */
    lastState = false

    /** @param  {HTMLElement} parent 
     *  @param  {HTMLElement} self 
     *  @param  {RegExp} regex 
    */
    constructor(parent, self, regex) {
        this.parent = parent;
        this.self = self;
        this.regex = regex;
        this.icon = self.querySelector('use');
        if( iconBaseUrl == ""){
            iconBaseUrl = this.icon.getAttribute('href').split('#')[0];
        }
    }
    /**
     * @param   {string} target 
     * @returns {Boolean}
     */
    check(target){
        const ok = this.regex.test(target);
        if( ok && !this.lastState){
            this.lastState = true;
            this.self.style.textDecoration = "line-through";
            this.icon.setAttribute('href', `${iconBaseUrl}#check`);
        }
        if( !ok && this.lastState){
            this.lastState = false;
            this.self.style.textDecoration = ""
            this.icon.setAttribute('href', `${iconBaseUrl}#cross`);
        }
        return ok;
    }
}

let iconBaseUrl = "";
function Init() {
    const passwordInput = document.getElementById('password-input');
    const passwordInputWrapper = document.getElementById('password-input-wrapper');
    const ruleListElement = document.getElementById('password-rule-list');
    if (passwordInput === null || ruleListElement === null) return;
    const ruleElements = ruleListElement.querySelectorAll('li');
    if (ruleElements.length <= 0) return;
    
    /** @type {Rule[]} */
    let rules = []
    ruleElements.forEach(ruleElement => {
        rules.push(new Rule(ruleListElement, ruleElement, RegExp(ruleElement.getAttribute('data-regex'))));
    });

    const focusinFunc = (e) =>{
        ruleListElement.classList.remove('hidden');
    }
    const focusoutFunc = (e) =>{
        ruleListElement.classList.add('hidden');
    }

    if( passwordInputWrapper === null ) {
        passwordInput.addEventListener('focusin',  focusinFunc);
        passwordInput.addEventListener('focusout', focusoutFunc);
    }else{
        passwordInputWrapper.addEventListener('focusin',  focusinFunc);
        passwordInputWrapper.addEventListener('focusout', focusoutFunc);
    }


    passwordInput.addEventListener('input', (e) =>{
        let ok = true;
        rules.forEach(rule => {
            if(!rule.check(passwordInput.value) && ok === true){
                ok = false;
            }
        });
    });
}
Init();
