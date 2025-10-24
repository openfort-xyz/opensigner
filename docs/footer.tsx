"use client"

import { useEffect, useState } from 'react'

export default function Footer() {
    const [isDark, setIsDark] = useState(false)
    
    useEffect(() => {
        const checkDarkMode = () => {
            setIsDark(document.documentElement.classList.contains('dark'))
        }
        
        checkDarkMode()
        
        const observer = new MutationObserver(checkDarkMode)
        observer.observe(document.documentElement, { 
            attributes: true, 
            attributeFilter: ['class'] 
        })
        
        return () => observer.disconnect()
    }, [])
    
    return (
      <div className="flex items-center justify-center gap-2 text-xs text-text-300 p-2">
        <div>Presented By</div>
        <img 
          src={isDark ? "/icons/openfort-logo.svg" : "/icons/openfort-logo-black.svg"}
          alt="Openfort Logo"
          className="h-16 w-auto"
          style={{ minWidth: '100px' }}
        />
      </div>
    )
  }